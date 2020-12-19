/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/perfhelper"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"arhat.dev/aranya/pkg/apis"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/controller/aranya"
	"arhat.dev/aranya/pkg/controller/edgedevice"
)

func NewAranyaCmd() *cobra.Command {
	var (
		appCtx       context.Context
		configFile   string
		config       = new(conf.Config)
		cliLogConfig = new(log.Config)
	)

	aranyaCmd := &cobra.Command{
		Use:           "aranya",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Use == "version" {
				return nil
			}

			var err error
			appCtx, err = conf.ReadConfig(cmd, &configFile, cliLogConfig, config)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(appCtx, config)
		},
	}

	flags := aranyaCmd.PersistentFlags()

	flags.StringVarP(&configFile, "config", "c", constant.DefaultAranyaConfigFile, "path to the aranya config file")

	flags.AddFlagSet(kubehelper.FlagsForControllerConfig("aranya", "", cliLogConfig, &config.Aranya.ControllerConfig))
	flags.AddFlagSet(conf.FlagsForAranyaAppConfig("", &config.Aranya))
	flags.AddFlagSet(conf.FlagsForVirtualnode("", &config.VirtualNode))

	aranyaCmd.AddCommand(newCheckCmd(&appCtx))

	return aranyaCmd
}

func run(appCtx context.Context, config *conf.Config) error {
	logger := log.Log.WithName("aranya")

	logger.I("creating kube client for initialization")
	kubeClient, _, err := config.Aranya.KubeClient.NewKubeClient(nil, false)
	if err != nil {
		return fmt.Errorf("failed to create kube client from kubeconfig: %w", err)
	}

	err = setupTelemetry(
		&config.Aranya.Metrics,
		&config.Aranya.PProf,
		&config.Aranya.Tracing,
	)
	if err != nil {
		return fmt.Errorf("failed to setup telemetry: %w", err)
	}

	logger.I("trying to find myself")
	podClient := kubeClient.CoreV1().Pods(envhelper.ThisPodNS())
	thisPod, err := podClient.Get(appCtx, envhelper.ThisPodName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get this pod itself: %w", err)
	}

	logger.I("trying to find host node")
	thisNode, err := kubeClient.CoreV1().Nodes().Get(appCtx, thisPod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get host node of this pod: %w", err)
	}

	var (
		nodeAddresses    []corev1.NodeAddress
		hostname, hostIP string
	)

	for i, addr := range thisNode.Status.Addresses {
		switch addr.Type {
		case corev1.NodeHostName:
			hostname = addr.Address
		case corev1.NodeInternalIP:
			hostIP = addr.Address
		case corev1.NodeExternalIP:
			if hostIP == "" {
				hostIP = addr.Address
			}
		}

		nodeAddresses = append(nodeAddresses, thisNode.Status.Addresses[i])
	}

	if len(nodeAddresses) == 0 {
		return fmt.Errorf("failed to get at least one node address")
	}

	// setup scheme for all custom resources
	logger.D("adding api scheme")
	err = apis.AddToScheme(scheme.Scheme)
	if err != nil {
		return fmt.Errorf("failed to add api scheme: %w", err)
	}

	logger.D("discovering api resources")
	preferredResources, err := kubeClient.Discovery().ServerPreferredResources()
	if err != nil {
		logger.I("failed to discover cluster preferred api, will try to fallback", log.Error(err))
	}

	if len(preferredResources) == 0 {
		logger.I("no api resource discovered")
		preferredResources = edgedevice.CheckAPIVersionFallback(kubeClient)
	} else {
		logger.D("found api resources", log.Any("resources", preferredResources))
	}

	logger.I("creating edge device controller")
	edCtrl, err := edgedevice.NewController(
		appCtx, config,
		thisNode.Name, hostname, hostIP,
		nodeAddresses,
		preferredResources,
	)
	if err != nil {
		return fmt.Errorf("failed to create edge device controller: %w", err)
	}

	logger.I("creating aranya controller")
	aranyaCtrl, err := aranya.NewController(appCtx, logger, kubeClient, edCtrl)
	if err != nil {
		return fmt.Errorf("failed to create aranya controller: %w", err)
	}

	logger.I("starting aranya controller")
	err = aranyaCtrl.Start()
	if err != nil {
		return fmt.Errorf("failed to start aranya controller: %w", err)
	}

	logger.V("creating leader elector")
	evb := record.NewBroadcaster()
	watchEventLogging := evb.StartLogging(func(format string, args ...interface{}) {
		logger.I(fmt.Sprintf(format, args...), log.String("source", "event"))
	})
	watchEventRecording := evb.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeClient.CoreV1().Events(envhelper.ThisPodNS()),
	})
	defer func() {
		watchEventLogging.Stop()
		watchEventRecording.Stop()
	}()

	elector, err := config.Aranya.LeaderElection.CreateElector("aranya", kubeClient,
		evb.NewRecorder(scheme.Scheme, corev1.EventSource{
			Component: "aranya",
		}),
		aranyaCtrl.OnElected,
		aranyaCtrl.OnEjected,
		aranyaCtrl.OnNewLeader,
	)
	if err != nil {
		return fmt.Errorf("failed to create elector: %w", err)
	}

	logger.I("running controller with leader election")
	elector.Run(appCtx)
	return nil
}

func setupTelemetry(
	metricsConfig *perfhelper.MetricsConfig,
	pprofConfig *perfhelper.PProfConfig,
	tracingConfig *perfhelper.TracingConfig,
) error {
	_, mtHandler, err := metricsConfig.CreateIfEnabled(true)
	if err != nil {
		return fmt.Errorf("failed to create metrics provider: %w", err)
	}

	pprofHandlers := pprofConfig.CreateHTTPHandlersIfEnabled(true)

	// reuse listen address if pprof and metrics using the same address
	reuse := false
	func() {
		if !metricsConfig.Enabled || !pprofConfig.Enabled {
			// not enabled
			return
		}

		if mtHandler == nil {
			// only prometheus format requires tcp listen
			return
		}

		if metricsConfig.Endpoint != pprofConfig.Listen {
			// has same listen address
			return
		}

		if metricsConfig.HTTPPath == pprofConfig.HTTPPath {
			// same path, should not reuse
			return
		}

		// TODO: compare tls config
		reuse = true
	}()

	if mtHandler != nil {
		mux := http.NewServeMux()
		mux.Handle(metricsConfig.HTTPPath, mtHandler)
		if reuse {
			basePath := strings.TrimRight(pprofConfig.HTTPPath, "/")
			for pprofTarget, handler := range pprofHandlers {
				mux.Handle(basePath+"/"+pprofTarget, handler)
			}
		}

		tlsConfig, err2 := metricsConfig.TLS.GetTLSConfig(true)
		if err2 != nil {
			return fmt.Errorf("failed to get tls config for metrics listener: %w", err2)
		}

		srv := &http.Server{
			Handler:   mux,
			Addr:      metricsConfig.Endpoint,
			TLSConfig: tlsConfig,
		}

		go func() {
			err2 = srv.ListenAndServe()
			if err2 != nil && !errors.Is(err2, http.ErrServerClosed) {
				panic(err2)
			}
		}()
	}

	if !reuse {
		mux := http.NewServeMux()
		basePath := strings.TrimRight(pprofConfig.HTTPPath, "/")
		for pprofTarget, handler := range pprofHandlers {
			mux.Handle(basePath+"/"+pprofTarget, handler)
		}

		tlsConfig, err2 := pprofConfig.TLS.GetTLSConfig(true)
		if err2 != nil {
			return fmt.Errorf("failed to get tls config for pprof listener: %w", err2)
		}

		srv := &http.Server{
			Handler:   mux,
			Addr:      pprofConfig.Listen,
			TLSConfig: tlsConfig,
		}

		go func() {
			err2 = srv.ListenAndServe()
			if err2 != nil && !errors.Is(err2, http.ErrServerClosed) {
				panic(err2)
			}
		}()
	}

	_, err = tracingConfig.CreateIfEnabled(true, nil)
	if err != nil {
		return fmt.Errorf("failed to create tracing provider: %w", err)
	}

	return nil
}
