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
	"fmt"
	"io"
	"os"
	"time"

	"arhat.dev/pkg/iohelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya-proto/aranyagopb"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/util/logutil"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type containerExecutor func(
	name string,
	uid types.UID,
	container string,
	cmd []string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
	timeout time.Duration,
) error

func (doExec containerExecutor) ExecInContainer(
	name string,
	uid types.UID,
	container string,
	cmd []string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
	timeout time.Duration,
) error {
	return doExec(name, uid, container, cmd, stdin, stdout, stderr, tty, resize, timeout)
}

type containerAttacher func(
	name string,
	uid types.UID,
	container string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
) error

func (doAttach containerAttacher) AttachContainer(
	name string,
	uid types.UID,
	container string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
) error {
	return doAttach(name, uid, container, stdin, stdout, stderr, tty, resize)
}

func (m *Manager) doGetContainerLogs(
	uid types.UID,
	podName, logPath string,
	options *corev1.PodLogOptions,
) (uint64, io.ReadCloser, error) {
	var (
		since      time.Time
		tailLines  int64 = -1
		bytesLimit int64 = -1
		cmd        *aranyagopb.PodOperationCmd
		podLog     string
		logger     = m.Log.WithFields(log.String("type", "logs"))
	)

	switch {
	case options != nil:
		if options.SinceTime != nil {
			since = options.SinceTime.Time
		} else if options.SinceSeconds != nil {
			since = time.Now().Add(-time.Duration(*options.SinceSeconds) * time.Second)
		}

		if options.TailLines != nil {
			tailLines = *options.TailLines
		}

		if options.LimitBytes != nil {
			bytesLimit = *options.LimitBytes
		}

		cmd = aranyagopb.NewPodLogCmd(
			string(uid),
			options.Container,
			options.Follow,
			options.Timestamps,
			options.Previous,
			since,
			tailLines,
			bytesLimit,
		)

		// try local file for virtual pod host exec containers
		podLog = containerLogFile(
			podLogDir(
				m.options.Config.LogDir,
				constant.WatchNS(),
				podName,
				string(uid),
			),
			options.Container,
		)
		f, err := os.Lstat(podLog)
		if err != nil || f.IsDir() {
			podLog = ""
		}
	case logPath != "":
		cmd = aranyagopb.NewHostLogCmd(logPath)
	default:
		return 0, nil, fmt.Errorf("bad log options")
	}

	if podLog != "" {
		reader, writer := iohelper.Pipe()
		go func() {
			defer func() {
				_ = writer.Close()
			}()

			err := logutil.ReadLogs(m.Context(), podLog, cmd.GetLogOptions(), writer, writer)
			if err != nil {
				logger.I("failed to read local pod logs", log.Error(err))
			}
		}()

		return 0, reader, nil
	}

	msgCh, sid, err := m.ConnectivityManager.PostCmd(0, cmd)
	if err != nil {
		logger.I("failed to establish session for logs", log.Error(err))
		return 0, nil, err
	}

	reader, writer := iohelper.Pipe()
	go func() {
		defer func() { _ = writer.Close() }()

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("error happened in pod logs", log.Error(msgErr))
			}

			return true
		}, func(dataMsg *aranyagopb.Data) (exit bool) {
			if dataMsg.Kind == aranyagopb.DATA_ERROR {
				msgErr := new(aranyagopb.Error)
				_ = msgErr.Unmarshal(dataMsg.Data)
				logger.I("error happened in pod logs", log.Error(msgErr))
				return true
			}

			if _, err := writer.Write(dataMsg.Data); err != nil {
				logger.I("failed to write log data", log.Error(err))
				return true
			}

			return false
		}, connectivity.HandleUnknownMessage(logger))
	}()

	return sid, reader, nil
}

func (m *Manager) doHandleExecInContainer() kubeletrc.Executor {
	return containerExecutor(func(
		name string,
		uid types.UID,
		container string,
		cmd []string,
		stdin io.Reader,
		stdout, stderr io.WriteCloser,
		tty bool,
		resize <-chan remotecommand.TerminalSize,
		timeout time.Duration,
	) error {
		defer func() {
			_ = stdout.Close()
			if stderr != nil {
				_ = stderr.Close()
			}
		}()

		// kubectl exec has no support for environment variables
		execCmd := aranyagopb.NewPodExecCmd(
			string(uid), container, cmd,
			stdin != nil, stdout != nil, stderr != nil,
			tty, nil,
		)
		err := m.doServeTerminalStream(execCmd, stdin, stdout, stderr, resize)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doHandleAttachContainer() kubeletrc.Attacher {
	return containerAttacher(func(
		name string,
		uid types.UID,
		container string,
		stdin io.Reader,
		stdout, stderr io.WriteCloser,
		tty bool,
		resize <-chan remotecommand.TerminalSize,
	) error {
		defer func() {
			_ = stdout.Close()
			if stderr != nil {
				_ = stderr.Close()
			}
		}()

		attachCmd := aranyagopb.NewPodAttachCmd(string(uid), container, stdin != nil, stdout != nil, stderr != nil, tty)
		err := m.doServeTerminalStream(attachCmd, stdin, stdout, stderr, resize)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doServeTerminalStream(
	initialCmd *aranyagopb.PodOperationCmd,
	stdin io.Reader, stdout, stderr io.Writer,
	resizeCh <-chan remotecommand.TerminalSize,
) error {
	logger := m.Log.WithFields(log.String("type", "terminal-stream"))

	if stdout == nil {
		return fmt.Errorf("output should not be nil")
	}

	msgCh, sid, err := m.ConnectivityManager.PostCmd(0, initialCmd)
	if err != nil {
		logger.I("failed to create session", log.Error(err))
		return err
	}

	logger = logger.WithFields(log.Uint64("sid", sid))

	defer func() {
		// best effort
		_, _, err2 := m.ConnectivityManager.PostCmd(sid, aranyagopb.NewSessionCloseCmd(sid))
		if err2 != nil {
			logger.I("failed to post session close cmd", log.Error(err2))
		}
	}()

	if resizeCh != nil {
		go func() {
			for size := range resizeCh {
				resizeCmd := aranyagopb.NewPodResizeCmd(size.Width, size.Height)
				if _, _, err2 := m.ConnectivityManager.PostCmd(sid, resizeCmd); err2 != nil {
					logger.I("failed to post resize cmd", log.Error(err2))
				}
			}
		}()
	}

	if stdin != nil {
		r := iohelper.NewTimeoutReader(stdin, m.ConnectivityManager.MaxDataSize())
		go r.StartBackgroundReading()

		readTimeout := constant.DefaultNonInteractiveStreamReadTimeout
		if resizeCh != nil {
			readTimeout = constant.DefaultInteractiveStreamReadTimeout
		}

		timer := time.NewTimer(0)
		if !timer.Stop() {
			<-timer.C
		}

		closeSig := make(chan struct{})
		defer func() {
			close(closeSig)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}()

		go func() {
			defer func() {
				logger.V("closing remote read")
				_, _, err2 := m.ConnectivityManager.PostCmd(sid, aranyagopb.NewPodInputCmd(true, nil))
				if err2 != nil {
					logger.I("failed to post input close cmd", log.Error(err2))
				}

				logger.D("finished terminal input")
			}()

			for r.WaitUntilHasData(closeSig) {
				timer.Reset(readTimeout)
				data, isTimeout := r.ReadUntilTimeout(timer.C)
				if !isTimeout && !timer.Stop() {
					<-timer.C
				}

				_, _, err2 := m.ConnectivityManager.PostCmd(sid, aranyagopb.NewPodInputCmd(false, data))
				if err2 != nil {
					logger.I("failed to post user input", log.Error(err2))
					return
				}
			}
		}()
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
		}
		return true
	}, func(dataMsg *aranyagopb.Data) (exit bool) {
		// default send to stdout
		targetOutput := stdout
		switch dataMsg.Kind {
		case aranyagopb.DATA_STDERR:
			if stderr != nil {
				targetOutput = stderr
			}
		case aranyagopb.DATA_ERROR:
			msgErr := new(aranyagopb.Error)
			_ = msgErr.Unmarshal(dataMsg.Data)
			err = msgErr
			return true
		}

		_, err = targetOutput.Write(dataMsg.Data)
		if err != nil && err != io.EOF {
			logger.I("failed to write output", log.Error(err))
			return true
		}
		return false
	}, connectivity.HandleUnknownMessage(logger))

	return err
}
