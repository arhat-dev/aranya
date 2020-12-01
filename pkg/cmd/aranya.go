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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"arhat.dev/pkg/encodehelper"
	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/patchhelper"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"arhat.dev/aranya/pkg/apis"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/controller/edgedevice"
	"arhat.dev/aranya/pkg/util/manager"
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

	metricsConfig := config.Aranya.Metrics
	pprofConfig := config.Aranya.PProf

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

	_, err = config.Aranya.Tracing.CreateIfEnabled(true, nil)
	if err != nil {
		return fmt.Errorf("failed to create tracing provider: %w", err)
	}

	var (
		privateKey  []byte
		sshListener net.Listener
		sftpManager *manager.SFTPManager
	)
	if config.VirtualNode.Storage.Enabled {
		sshListener, err = net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("failed to listen for ssh server: %w", err)
		}

		var (
			hostKeyBytes []byte
			hostKey      ssh.Signer
			pubKey       ed25519.PublicKey
			privKey      ed25519.PrivateKey
			publicKey    ssh.PublicKey
		)

		hostKeyBytes, err = ioutil.ReadFile(config.VirtualNode.Storage.SFTP.HostKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read sftp host key: %w", err)
		}

		hostKey, err = ssh.ParsePrivateKey(hostKeyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse sftp host key: %w", err)
		}

		pubKey, privKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fmt.Errorf("failed to generate ed25519 key pair: %w", err)
		}

		publicKey, _ = ssh.NewPublicKey(pubKey)
		privateKey = pem.EncodeToMemory(
			&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: encodehelper.MarshalED25519PrivateKeyForOpenSSH(privKey)})

		sftpManager = manager.NewSFTPManager(
			appCtx, config.VirtualNode.Storage.RootDir, sshListener, hostKey, publicKey.Marshal())
	}

	evb := record.NewBroadcaster()
	watchEventLogging := evb.StartLogging(func(format string, args ...interface{}) {
		logger.I(fmt.Sprintf(format, args...), log.String("source", "event"))
	})
	watchEventRecording := evb.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events(envhelper.ThisPodNS())})
	defer func() {
		watchEventLogging.Stop()
		watchEventRecording.Stop()
	}()

	logger.V("creating leader elector")
	elector, err := config.Aranya.LeaderElection.CreateElector("aranya", kubeClient,
		evb.NewRecorder(scheme.Scheme, corev1.EventSource{
			Component: "aranya",
		}),
		// on elected
		func(ctx context.Context) {
			err2 := onElected(ctx, logger, kubeClient, config, sshListener, sftpManager, privateKey)
			if err2 != nil {
				logger.E("failed to run controller", log.Error(err2))
				os.Exit(1)
			}
		},
		// on ejected
		func() {
			logger.E("lost leader-election")
			os.Exit(1)
		},
		// on new leader
		func(identity string) {
			// TODO: react when new leader elected
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create elector: %w", err)
	}

	logger.I("running leader election")
	elector.Run(appCtx)

	return fmt.Errorf("unreachable code")
}

func onElected(
	appCtx context.Context,
	logger log.Interface,
	kubeClient kubeclient.Interface,
	config *conf.Config,

	// storage related
	sshListener net.Listener,
	sftpManager *manager.SFTPManager,
	privateKey []byte,
) error {
	logger.I("won leader election")

	logger.I("trying to find myself")
	podClient := kubeClient.CoreV1().Pods(envhelper.ThisPodNS())
	thisPod, err := podClient.Get(appCtx, envhelper.ThisPodName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get this pod itself: %w", err)
	}

	logger.I("trying to find host node I am living on")
	thisNode, err := kubeClient.CoreV1().Nodes().Get(appCtx, thisPod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get host node of this pod: %w", err)
	}

	podLabels := make(map[string]string)
	for i := 0; i < 5; i++ {
		if thisPod.Labels != nil {
			podLabels = thisPod.Labels
		}
		podLabels[constant.LabelControllerLeadership] = constant.LabelControllerLeadershipLeader
		thisPod.Labels = podLabels

		_, err = podClient.Update(appCtx, thisPod, metav1.UpdateOptions{})
		if err == nil {
			break
		}

		logger.I("failed to add master leadership label to this pod", log.Error(err))

		if !kubeerrors.IsConflict(err) {
			continue
		}

		// conflict, get latest pod object to update
		thisPod, err = podClient.Get(appCtx, envhelper.ThisPodName(), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get this pod itself for conflicted update: %w", err)
		}
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

	if config.VirtualNode.Storage.SFTP.Enabled {
		err = setupServiceForSftpManager(
			config.Aranya.Managed.SFTPService.Name, sshListener.Addr().String(), kubeClient, podLabels,
		)
		if err != nil {
			return fmt.Errorf("failed to setup sftp service: %w", err)
		}

		logger.D("starting sftp manager")
		go func() {
			if err2 := sftpManager.Start(); err2 != nil {
				logger.E("failed to start sftp manager", log.Error(err2))
				os.Exit(1)
			}
		}()
	}

	// setup scheme for all custom resources
	logger.D("adding api scheme")
	if err = apis.AddToScheme(scheme.Scheme); err != nil {
		return fmt.Errorf("failed to add api scheme: %w", err)
	}

	logger.D("discovering api resources")
	preferredResources, err := kubeClient.Discovery().ServerPreferredResources()
	if err != nil {
		logger.I("failed to finish api discovery", log.Error(err))
	}

	if len(preferredResources) == 0 {
		logger.I("no api resource discovered")
		preferredResources = edgedevice.CheckAPIVersionFallback(kubeClient)
	} else {
		logger.D("found api resources", log.Any("resources", preferredResources))
	}

	// create and start controller
	logger.I("creating controller")
	controller, err := edgedevice.NewController(
		appCtx, config, privateKey,
		thisNode.Name, hostname, hostIP, nodeAddresses,
		podLabels,
		preferredResources,
	)
	if err != nil {
		return fmt.Errorf("failed to create new controller: %w", err)
	}

	logger.I("starting controller")
	if err = controller.Start(); err != nil {
		return fmt.Errorf("failed to start controller: %w", err)
	}

	return fmt.Errorf("controller exited: %w", appCtx.Err())
}

func setupServiceForSftpManager(
	name, listenAddr string,
	kubeClient kubeclient.Interface,
	selector map[string]string,
) error {
	if name == "" {
		// no service will be managed by us
		return nil
	}

	_, portStr, _ := net.SplitHostPort(listenAddr)
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return fmt.Errorf("failed to parse sftp listener port value: %w", err)
	}

	create := false
	svc := newServiceForSFTP(name, int(port), selector)
	svcClient := kubeClient.CoreV1().Services(envhelper.ThisPodNS())

	oldSvc, err := svcClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		// try to patch update
		clone := oldSvc.DeepCopy()

		clone.Spec.Ports = svc.Spec.Ports
		clone.Spec.Selector = svc.Spec.Selector
		clone.Spec.ClusterIP = svc.Spec.ClusterIP

		err = patchhelper.TwoWayMergePatch(oldSvc, clone, &corev1.Service{}, func(patchData []byte) error {
			_, err2 := svcClient.Patch(
				context.TODO(), name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
			if err2 != nil {
				return fmt.Errorf("failed to patch node spec: %w", err2)
			}
			return nil
		})

		if err != nil {
			err = svcClient.Delete(context.TODO(), name, *metav1.NewDeleteOptions(0))
			if err != nil && !kubeerrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete managed sftp service")
			}

			create = true
		}
	} else {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get old sftp service: %w", err)
		}

		create = true
	}

	if create {
		_, err := svcClient.Create(context.TODO(), svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create managed sftp service: %w", err)
		}
	}

	return nil
}

func newServiceForSFTP(name string, listenerPort int, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: envhelper.ThisPodNS(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "sftp",
				Port:       22,
				TargetPort: intstr.FromInt(listenerPort),
			}},
			Selector:  selector,
			ClusterIP: corev1.ClusterIPNone,
		},
	}
}
