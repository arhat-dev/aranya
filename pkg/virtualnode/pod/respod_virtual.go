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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"arhat.dev/pkg/iohelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya/pkg/constant"
)

func (m *Manager) updateVirtualPodToRunningPhase() error {
	pod := m.options.GetPod(m.nodeName)
	if pod == nil {
		return fmt.Errorf("failed to get virtual pod")
	}

	now := metav1.Now()
	status := new(corev1.PodStatus)

	status.Phase = corev1.PodRunning
	for _, ctr := range pod.Spec.Containers {
		t := true
		status.ContainerStatuses = append(status.ContainerStatuses, corev1.ContainerStatus{
			Name:                 ctr.Name,
			State:                corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: now}},
			LastTerminationState: corev1.ContainerState{},
			Ready:                true,
			Started:              &t,
			RestartCount:         0,
			Image: func() string {
				switch ctr.Name {
				case constant.VirtualContainerNameHost:
					return constant.VirtualImageNameHost
				default:
					return constant.VirtualImageNameDevice
				}
			}(),
			ImageID: func() string {
				switch ctr.Name {
				case constant.VirtualContainerNameHost:
					return constant.VirtualImageIDHost
				default:
					return constant.VirtualImageIDDevice
				}
			}(),
			ContainerID: func() string {
				switch ctr.Name {
				case constant.VirtualContainerNameHost:
					return constant.VirtualContainerIDHost
				default:
					return constant.VirtualContainerIDDevice
				}
			}(),
		})
	}

	status.Conditions = []corev1.PodCondition{{
		Type:               corev1.ContainersReady,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      now,
		LastTransitionTime: now,
		Reason:             "Online",
		Message:            "edge device online",
	}, {
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      now,
		LastTransitionTime: now,
		Reason:             "Online",
		Message:            "edge device online",
	}}

	pod.Status = *status

	_, err := m.UpdatePodStatus(pod)
	if err != nil {
		return fmt.Errorf("failed to update virtual pod status: %w", err)
	}

	return nil
}

// only return error for log file related errors
func (m *Manager) handleContainerAsHostExec(opts *aranyagopb.CreateOptions) (*aranyagopb.PodStatus, error) {
	// defensive, should not happen
	if opts.PodUid == "" || opts.Namespace == "" || opts.Name == "" {
		return nil, fmt.Errorf("invalid options: pod uid should not be empty")
	}

	containerStatus := make(map[string]*aranyagopb.PodStatus_ContainerStatus)

	// the real log path is different from the one documented here:
	//    https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/kubelet-cri-logging.md
	//
	// default kubelet pod container log path:
	//    /var/log/pods/<namespace>_<pod-name>_<pod-uid>/<container-name>/{1...N}.log
	//
	podLogDir := podLogDir(m.options.Config.LogDir, opts.Namespace, opts.Name, opts.PodUid)
	logFiles := make(map[string]*os.File)

	defer func() {
		for _, f := range logFiles {
			if f != nil {
				_ = f.Close()
			}
		}
	}()

	for _, ctr := range opts.Containers {
		var (
			logFilePath = containerLogFile(podLogDir, ctr.Name)
			logDir      = filepath.Dir(logFilePath)
		)

		err := os.MkdirAll(logDir, 0755)
		if err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("failed to create container log dir %q: %w", logDir, err)
		}

		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_SYNC, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create pod container log file %q: %w", logFilePath, err)
		}
		logFiles[ctr.Name] = logFile
	}

	for _, ctr := range opts.Containers {
		// create log dirs for pod containers
		var (
			logFile = logFiles[ctr.Name]
			logger  = m.Log.WithFields(log.String("pod", opts.Name), log.String("ctr", ctr.Name))
		)

		cmd := append([]string{}, ctr.Command...)
		createdAt := time.Now().UTC().Format(constant.TimeLayout)

		var err error
		for _, arg := range ctr.Args {
			execCmd := aranyagopb.NewPodExecCmd("", constant.VirtualContainerNameHost,
				append(append([]string{}, cmd...), arg), ctr.Stdin, true, true, ctr.Tty, ctr.Envs)

			stderrR, stderr := iohelper.Pipe()
			stdoutR, stdout := iohelper.Pipe()

			logger.V("Executing: " + arg)

			go func() {
				defer func() {
					_ = stdoutR.Close()
				}()

				s := bufio.NewScanner(stdoutR)
				s.Split(bufio.ScanLines)

				for s.Scan() {
					_, _ = logFile.WriteString(
						time.Now().UTC().Format(time.RFC3339Nano) + " stdout F " + s.Text() + "\n")
				}
			}()

			go func() {
				defer func() {
					_ = stderrR.Close()
				}()

				s := bufio.NewScanner(stderrR)
				s.Split(bufio.ScanLines)
				for s.Scan() {
					// TODO: investigate whether stderr logging is not working
					_, _ = logFile.WriteString(
						time.Now().UTC().Format(time.RFC3339Nano) + " stderr F " + s.Text() + "\n")
				}
			}()

			err = func() error {
				defer func() {
					_, _ = stdout.Close(), stderr.Close()
				}()

				return m.doServeTerminalStream(execCmd, nil, stdout, stderr, nil)
			}()

			if err != nil {
				break
			}
		}

		finishedAt := time.Now().UTC().Format(constant.TimeLayout)

		ctrStatus := &aranyagopb.PodStatus_ContainerStatus{
			//ContainerId:  "",
			//ImageId:      "",
			CreatedAt: createdAt,
			// TODO: record correct startedAt
			StartedAt:    createdAt,
			FinishedAt:   finishedAt,
			RestartCount: 0,
		}

		if err != nil {
			ctrStatus.Message = "CommandExecFailed"

			if msgErr, ok := err.(*aranyagopb.Error); ok {
				ctrStatus.ExitCode = int32(msgErr.Code)
				ctrStatus.Reason = msgErr.Description
			} else {
				ctrStatus.ExitCode = constant.DefaultExitCodeOnError
				ctrStatus.Reason = err.Error()
			}
		}

		containerStatus[ctr.Name] = ctrStatus

		if err != nil {
			break
		}
	}

	return aranyagopb.NewPodStatus(opts.PodUid, "", containerStatus), nil
}
