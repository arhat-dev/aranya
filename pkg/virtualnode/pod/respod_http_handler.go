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

package pod

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"arhat.dev/pkg/log"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	rcconst "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apiserver/pkg/util/flushwriter"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	_ "k8s.io/kubernetes/pkg/apis/core/install" // install legacyscheme
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
	kubeletpf "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya-proto/gopb"
	"arhat.dev/aranya/pkg/constant"
)

// PathParams http path var
const (
	PathParamNamespace = "namespace"
	PathParamPodName   = "name"
	PathParamPodUID    = "uid"
	PathParamContainer = "container"
)

func getParamsForExec(req *http.Request) (podName string, uid types.UID, containerName string, command []string) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID]),
		pathVars[PathParamContainer], req.URL.Query()[corev1.ExecCommandParam]
}

func getParamsForPortForward(req *http.Request) (podName string, uid types.UID) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID])
}

func getParamsForContainerLog(req *http.Request) (podName string, logOptions *corev1.PodLogOptions, err error) {
	pathVars := mux.Vars(req)

	podName = pathVars[PathParamPodName]
	if podName == "" {
		err = errors.New("missing pod name")
		return
	}

	containerName := pathVars[PathParamContainer]
	if containerName == "" {
		err = errors.New("missing container name")
		return
	}

	query := req.URL.Query()
	// backwards compatibility for the "tail" query parameter
	if tail := req.FormValue("tail"); len(tail) > 0 {
		query["tailLines"] = []string{tail}
		// "all" is the same as omitting tail
		if tail == "all" {
			delete(query, "tailLines")
		}
	}
	query.Get("tailLines")

	// container logs on the kubelet are locked to the v1 API version of PodLogOptions
	logOptions = &corev1.PodLogOptions{}
	if err = legacyscheme.ParameterCodec.DecodeParameters(query, corev1.SchemeGroupVersion, logOptions); err != nil {
		return
	}

	logOptions.TypeMeta = metav1.TypeMeta{}
	if errs := validation.ValidatePodLogOptions(logOptions); len(errs) > 0 {
		err = errors.New("invalid request")
		return
	}

	logOptions.Container = containerName
	return
}

func getRemoteCommandOptions(req *http.Request) *kubeletrc.Options {
	return &kubeletrc.Options{
		TTY:    req.FormValue(corev1.ExecTTYParam) == "1",
		Stdin:  req.FormValue(corev1.ExecStdinParam) == "1",
		Stdout: req.FormValue(corev1.ExecStdoutParam) == "1",
		Stderr: req.FormValue(corev1.ExecStderrParam) == "1",
	}
}

func (m *Manager) getPodUIDInCache(name string, podUID types.UID) types.UID {
	if podUID != "" {
		return podUID
	}

	pod, ok := m.podCache.GetByName(constant.WatchNS(), name)
	if ok {
		return pod.UID
	}
	return ""
}

func (m *Manager) HandleHostLog(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "nodeLog"))
	logPath := strings.TrimPrefix(r.URL.Path, "/logs/")

	logger.D("serving host logs", log.String("path", logPath))
	_, logReader, err := m.doGetContainerLogs("", "", logPath, nil)
	if err != nil {
		logger.I("failed to get host logs", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = logReader.Close() }()

	s := bufio.NewScanner(logReader)
	s.Split(bufio.ScanLines)

	_ = s.Scan()
	firstLine := s.Text()
	switch firstLine {
	case constant.IdentifierLogDir:
		_, err = w.Write([]byte("<pre>\n"))
		if err != nil {
			break
		}

		for s.Scan() {
			line := s.Text()
			_, err = w.Write([]byte(fmt.Sprintf("<a href=\"%s\">%s</a>\n", line, line)))
			if err != nil {
				break
			}
		}

		_, err = w.Write([]byte("</pre>\n"))
	case constant.IdentifierLogFile:
		_, err = io.Copy(w, logReader)
	default:
		http.Error(w, "unknown log result type", http.StatusInternalServerError)
		logger.I("bad first line, unknown type", log.String("firstLine", firstLine))
	}

	if err != nil {
		logger.I("failed to write host log response", log.Error(err))
		return
	}
}

// HandlePodLog proxy http based kubectl logs command to edge device
func (m *Manager) HandlePodLog(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "podLog"))

	logger.D("resolving log options")
	podName, opt, err := getParamsForContainerLog(r)
	if err != nil {
		logger.I("failed to parse container log options", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var podUID types.UID
	if podName == m.nodeName {
		// get arhat log
		podUID = ""
	} else {
		podUID = m.getPodUIDInCache(podName, "")
		if podUID == "" {
			logger.I("pod not found", log.String("podUID", string(podUID)))
			http.Error(w, "pod not found", http.StatusNotFound)
			return
		}
	}

	logger.D("serving container logs")
	sid, logReader, err := m.doGetContainerLogs(podUID, podName, "", opt)
	if err != nil {
		logger.I("failed to fetch container logs", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = logReader.Close() }()

	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, fmt.Sprintf("unable to convert %v into http.Flusher, cannot show logs",
			reflect.TypeOf(w)), http.StatusInternalServerError)
		return
	}

	if sid != 0 {
		defer func() {
			// best effort to close logging in edge device
			_, _, err := m.ConnectivityManager.PostCmd(sid, gopb.NewSessionCloseCmd(sid))
			if err != nil {
				logger.I("failed to post log session close cmd", log.Error(err))
			}
		}()
	}

	fw := flushwriter.Wrap(w)
	w.Header().Set("Transfer-Encoding", "chunked")
	if _, err := io.Copy(fw, logReader); err != nil {
		logger.I("failed to write container log response", log.Error(err))
		return
	}
}

// HandlePodExec proxy http based kubectl exec command to edge device
func (m *Manager) HandlePodExec(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "exec"))

	logger.D("resolving exec options")
	podName, uid, containerName, cmd := getParamsForExec(r)

	var podUID types.UID
	if podName == m.nodeName {
		// exec in host
		podUID = ""
	} else {
		podUID = m.getPodUIDInCache(podName, uid)
		if podUID == "" {
			logger.I("pod not found for exec", log.String("podUID", string(podUID)))
			http.Error(w, "target pod not found", http.StatusNotFound)
			return
		}
	}

	logger.D("starting to serve exec")
	kubeletrc.ServeExec(
		w, r, /* http context */
		m.doHandleExecInContainer(), /* wrapped pod executor */
		"",                          /* pod name (unused) */
		podUID,                      /* unique id of pod */
		containerName,               /* container name to execute in*/
		cmd,                         /* commands to execute */
		getRemoteCommandOptions(r),  /* stream options */
		// timeout options
		m.options.Config.Timers.StreamIdleTimeout, m.options.Config.Timers.StreamCreationTimeout,
		// supported protocols
		rcconst.SupportedStreamingProtocols)
}

// HandlePodAttach proxy http based kubectl attach command to edge device
func (m *Manager) HandlePodAttach(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "attach"))

	logger.D("resolving attach options")
	podName, uid, containerName, _ := getParamsForExec(r)

	var podUID types.UID
	if podName == m.nodeName {
		// attach to host
		podUID = ""
	} else {
		podUID = m.getPodUIDInCache(podName, uid)
		if podUID == "" {
			logger.I("pod not found for attach", log.String("podUID", string(podUID)))
			http.Error(w, "target pod not found", http.StatusNotFound)
			return
		}
	}

	logger.D("serving container attach")
	kubeletrc.ServeAttach(
		w, r, /* http context */
		m.doHandleAttachContainer(), /* wrapped pod attacher */
		"",                          /* pod name (not used) */
		podUID,                      /* unique id of pod */
		containerName,               /* container to execute in */
		getRemoteCommandOptions(r),  /* stream options */
		// timeout options
		m.options.Config.Timers.StreamIdleTimeout, m.options.Config.Timers.StreamCreationTimeout,
		// supported protocols
		rcconst.SupportedStreamingProtocols)
}

// HandlePodPortForward proxy http based kubectl port-forward command to edge device
func (m *Manager) HandlePodPortForward(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "portforward"))

	podName, uid := getParamsForPortForward(r)
	logger.D("resolving port-forward options")
	portForwardOptions, err := kubeletpf.NewV4Options(r)
	if err != nil {
		logger.I("failed to parse port-forward options", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var podUID types.UID
	if podName == m.nodeName {
		// forward to host
		podUID = ""
	} else {
		podUID = m.getPodUIDInCache(podName, uid)
		if podUID == "" {
			logger.I("pod not found for port forward", log.String("podUID", string(podUID)))
			http.Error(w, "target pod not found", http.StatusNotFound)
			return
		}
	}

	// build port protocol map
	pod, ok := m.podCache.GetByID(podUID)
	if podUID != "" && !ok {
		logger.I("pod not found", log.String("podUID", string(podUID)))
		http.Error(w, "pod not found", http.StatusNotFound)
		return
	}

	portProto := make(map[int32]string)
	for _, port := range portForwardOptions.Ports {
		// defaults to tcp
		portProto[port] = "tcp"
	}

	if pod != nil {
		for _, ctr := range pod.Spec.Containers {
			for _, ctrPort := range ctr.Ports {
				portProto[ctrPort.ContainerPort] = strings.ToLower(string(ctrPort.Protocol))
			}
		}
	}

	logger.D("serving port forward")
	err = m.serverPortForward(
		w, r, /* http context */
		string(podUID),     /* unique id of pod */
		portForwardOptions, /* port forward options (ports) */
		// timeout options
		m.options.Config.Timers.StreamIdleTimeout, m.options.Config.Timers.StreamCreationTimeout,
		// supported protocols
		kubeletpf.SupportedProtocols)

	if err != nil {
		logger.I("failed to serve portforward", log.Error(err))
	}
}

func (m *Manager) HandleGetPods(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "getPods"))
	logger.D("serving pods")
	m.writePodsResp(logger, m.podCache.GetAll(), w)
}

func (m *Manager) HandleGetRunningPods(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "getRunningPods"))
	allPods := m.podCache.GetAll()
	var pods []*corev1.Pod
	for _, pod := range allPods {
		if pod.Status.Phase == corev1.PodRunning {
			pods = append(pods, pod)
		}
	}

	logger.D("serving running pods")
	m.writePodsResp(logger, pods, w)
}

func (m *Manager) writePodsResp(logger log.Interface, pods []*corev1.Pod, w http.ResponseWriter) {
	podList := new(corev1.PodList)
	for _, pod := range pods {
		podList.Items = append(podList.Items, *pod)
	}

	codec := legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: corev1.GroupName, Version: "v1"})
	data, err := runtime.Encode(codec, podList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if data == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		logger.I("failed to write response", log.Error(err))
	}
}
