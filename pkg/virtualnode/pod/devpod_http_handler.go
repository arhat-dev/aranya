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
	"io/ioutil"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/iohelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/cri/streaming/remotecommand"

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
		cmd        *aranyagopb.LogsCmd
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

		cmd = aranyagopb.NewLogsCmd(
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

			err := logutil.ReadLogs(m.Context(), podLog, cmd, writer, writer)
			if err != nil {
				logger.I("failed to read local pod logs", log.Error(err))
			}
		}()

		return 0, reader, nil
	}

	msgCh, sid, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_LOGS, cmd)
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
		}, func(data *connectivity.Data) (exit bool) {
			if _, err := writer.Write(data.Payload); err != nil {
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

		if len(cmd) == 0 {
			return fmt.Errorf("invalid empty command")
		}

		if uid == "" && container != constant.VirtualContainerNameHost {
			// manual device operation
			data := []byte(strings.Join(cmd[1:], " "))
			if stdin != nil {
				var err error
				data, err = ioutil.ReadAll(stdin)
				if err != nil {
					return fmt.Errorf("failed to read all stdin data: %w", err)
				}
			}

			err := m.options.OperateDevice(container, cmd[0], data, stdout)
			if err != nil {
				return fmt.Errorf("manual device operation failed: %w", err)
			}

			return nil
		}

		// kubectl exec has no support for environment variables
		execCmd := aranyagopb.NewExecCmd(
			string(uid), container, cmd,
			stdin != nil, stdout != nil, stderr != nil,
			tty, nil,
		)
		err := m.doServeTerminalStream(aranyagopb.CMD_EXEC, execCmd, stdin, stdout, stderr, resize)
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

		if uid == "" && container != constant.VirtualContainerNameHost {
			// manual device metrics collection
			err := m.options.CollectDeviceMetrics(container)
			if err != nil {
				return fmt.Errorf("manual device metrics collection failed: %w", err)
			}
		}

		attachCmd := aranyagopb.NewAttachCmd(
			string(uid), container,
			stdin != nil, stdout != nil, stderr != nil,
			tty,
		)
		err := m.doServeTerminalStream(aranyagopb.CMD_ATTACH, attachCmd, stdin, stdout, stderr, resize)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doServeTerminalStream(
	kind aranyagopb.CmdType,
	initialCmd *aranyagopb.ExecOrAttachCmd,
	stdin io.Reader, stdout, stderr io.Writer,
	resizeCh <-chan remotecommand.TerminalSize,
) error {
	logger := m.Log.WithFields(log.String("type", "terminal-stream"))

	if stdout == nil {
		return fmt.Errorf("output should not be nil")
	}

	msgCh, sid, err := m.ConnectivityManager.PostCmd(0, kind, initialCmd)
	if err != nil {
		logger.I("failed to create session", log.Error(err))
		return err
	}

	logger = logger.WithFields(log.Uint64("sid", sid))

	defer func() {
		// best effort
		_, _, err2 := m.ConnectivityManager.PostCmd(
			sid, aranyagopb.CMD_SESSION_CLOSE, aranyagopb.NewSessionCloseCmd(sid),
		)
		if err2 != nil {
			logger.I("failed to post session close cmd", log.Error(err2))
		}
	}()

	if resizeCh != nil {
		go func() {
			for size := range resizeCh {
				resizeCmd := aranyagopb.NewTerminalResizeCmd(size.Width, size.Height)
				_, _, err2 := m.ConnectivityManager.PostCmd(sid, aranyagopb.CMD_TTY_RESIZE, resizeCmd)
				if err2 != nil {
					logger.I("failed to post resize cmd", log.Error(err2))
				}
			}
		}()
	}

	if stdin != nil {
		readTimeout := constant.DefaultNonInteractiveStreamReadTimeout
		if resizeCh != nil {
			readTimeout = constant.DefaultInteractiveStreamReadTimeout
		}

		var (
			seq      uint64
			closeSig = make(chan struct{})
		)

		go func() {
			defer func() {
				logger.V("closing remote read")
				_, _, _, err2 := m.ConnectivityManager.PostData(
					sid, aranyagopb.CMD_DATA_UPSTREAM, nextSeq(&seq), true, nil,
				)
				if err2 != nil {
					logger.I("failed to post input close cmd", log.Error(err2))
				}

				logger.D("finished terminal input")
			}()

			r := iohelper.NewTimeoutReader(stdin)
			go r.FallbackReading(closeSig)

			buf := make([]byte, m.ConnectivityManager.MaxPayloadSize())
			for r.WaitForData(closeSig) {
				data, shouldCopy, err2 := r.Read(readTimeout, buf)
				if err2 != nil {
					if len(data) == 0 && err2 != iohelper.ErrDeadlineExceeded {
						break
					}
				}

				if shouldCopy {
					data = make([]byte, len(data))
					_ = copy(data, buf[:len(data)])
				}

				_, _, lastSeq, err2 := m.ConnectivityManager.PostData(
					sid, aranyagopb.CMD_DATA_UPSTREAM, nextSeq(&seq), false, data,
				)

				atomic.StoreUint64(&seq, lastSeq+1)
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
	}, func(dataMsg *connectivity.Data) (exit bool) {
		// default send to stdout
		targetOutput := stdout
		if dataMsg.Kind == aranyagopb.MSG_DATA_STDERR && stderr != nil {
			targetOutput = stderr
		}

		_, err = targetOutput.Write(dataMsg.Payload)
		if err != nil && err != io.EOF {
			logger.I("failed to write output", log.Error(err))
			return true
		}
		return false
	}, connectivity.HandleUnknownMessage(logger))

	return err
}

func nextSeq(p *uint64) uint64 {
	seq := atomic.LoadUint64(p)
	for !atomic.CompareAndSwapUint64(p, seq, seq+1) {
		seq++
	}

	return seq
}
