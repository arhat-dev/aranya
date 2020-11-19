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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
	"arhat.dev/pkg/iohelper"
	"arhat.dev/pkg/log"
	"ext.arhat.dev/runtimeutil/actionutil"
	"github.com/gogo/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/cri/streaming/remotecommand"

	"arhat.dev/aranya/pkg/constant"
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

func (m *Manager) doHandleLogs(
	uid types.UID,
	podName, logPath string,
	options *corev1.PodLogOptions,
	httpWriter io.Writer,
) error {
	var (
		since      time.Time
		tailLines  int64 = -1
		bytesLimit int64 = -1
		kind             = aranyagopb.CMD_LOGS
		cmd        proto.Marshaler
		logger     = m.Log.WithFields(log.String("type", "logs"))

		handleData = func(data []byte) error {
			_, err := httpWriter.Write(data)
			return err
		}
	)

	switch {
	case options != nil:
		// is log options for arhat or containers
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

		logsCmd := &aranyagopb.LogsCmd{
			PodUid:     string(uid),
			Container:  options.Container,
			Follow:     options.Follow,
			Timestamp:  options.Timestamps,
			Since:      since.Format(aranyagoconst.TimeLayout),
			TailLines:  tailLines,
			BytesLimit: bytesLimit,
			Previous:   options.Previous,
		}

		if uid == "" {
			// is host arhat log
			cmd = logsCmd
			break
		}

		// is runtime container log
		// try local file for virtual pod host exec containers
		podLog := containerLogFile(
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
			// is container log request for edge device
			kind = aranyagopb.CMD_RUNTIME

			data, err2 := logsCmd.Marshal()
			if err2 != nil {
				return fmt.Errorf("failed to marshal log options: %w", err2)
			}

			cmd = &runtimepb.Packet{
				Kind:    runtimepb.CMD_LOGS,
				Payload: data,
			}
			break
		}

		// is virtual pod host exec container log
		err = actionutil.ReadLogs(m.Context(), podLog, logsCmd, httpWriter, httpWriter)
		if err != nil {
			logger.I("failed to read local pod logs", log.Error(err))
		}

		return nil
	case logPath != "":
		// is host log
		cmd = &aranyagopb.LogsCmd{
			Path: logPath,
		}
		pr, pw := iohelper.Pipe()
		handleData = func(data []byte) error {
			_, err2 := pw.Write(data)
			return err2
		}

		go func() {
			s := bufio.NewScanner(pr)
			s.Split(bufio.ScanLines)

			_ = s.Scan()
			firstLine := s.Text()

			var err2 error
			switch firstLine {
			case constant.IdentifierLogDir:
				_, err2 = httpWriter.Write([]byte("<pre>\n"))
				if err2 != nil {
					break
				}

				for s.Scan() {
					line := s.Text()
					_, err2 = httpWriter.Write([]byte(fmt.Sprintf("<a href=\"%s\">%s</a>\n", line, line)))
					if err2 != nil {
						break
					}
				}
				if err2 != nil {
					break
				}

				_, err2 = httpWriter.Write([]byte("</pre>\n"))
			case constant.IdentifierLogFile:
				_, err2 = io.Copy(httpWriter, pr)
			default:
				http.Error(httpWriter.(http.ResponseWriter), "unknown log result type", http.StatusInternalServerError)
				logger.I("bad first line, unknown type", log.String("firstLine", firstLine))
			}

			if err2 != nil {
				http.Error(
					httpWriter.(http.ResponseWriter),
					fmt.Sprintf("partial logs failure: %v", err2),
					http.StatusInternalServerError,
				)
			}
		}()
	default:
		return fmt.Errorf("bad log options")
	}

	msgCh, sid, err := m.ConnectivityManager.PostCmd(0, kind, cmd)
	if err != nil {
		logger.I("failed to post log cmd", log.Error(err))
		return err
	}

	defer func() {
		// best effort to close logging in edge device
		_, _, err2 := m.ConnectivityManager.PostCmd(
			sid, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{Sid: sid},
		)
		if err2 != nil {
			logger.I("failed to post log session close cmd", log.Error(err2))
		}
	}()

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
			logger.I("error happened in pod logs", log.Error(msgErr))
		}

		return true
	}, func(data *connectivity.Data) (exit bool) {
		err2 := handleData(data.Payload)
		if err2 != nil {
			logger.I("failed to write log data", log.Error(err2))
			return true
		}

		return false
	}, connectivity.LogUnknownMessage(logger))

	return err
}

func (m *Manager) doHandleExec() kubeletrc.Executor {
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

		execCmd := &aranyagopb.ExecOrAttachCmd{
			Stdin:   stdin != nil,
			Stdout:  stdout != nil,
			Stderr:  stderr != nil,
			Tty:     tty,
			Command: cmd,
			Envs:    nil, // currently kubectl exec has no support for environment variables
		}

		if uid != "" {
			// is runtime pod exec

			execCmd.PodUid = string(uid)
			execCmd.Container = container

			data, err := execCmd.Marshal()
			if err != nil {
				return fmt.Errorf("failed to marshal pod exec options: %w", err)
			}

			return m.doServeTerminalStream(aranyagopb.CMD_RUNTIME, &runtimepb.Packet{
				Kind:    runtimepb.CMD_EXEC,
				Payload: data,
			}, stdin, stdout, stderr, resize)
		}

		// is special purpose exec
		switch container {
		case constant.VirtualContainerNameHost:
			// is host exec
			return m.doServeTerminalStream(aranyagopb.CMD_EXEC, execCmd, stdin, stdout, stderr, resize)
		default:
			// is peripheral operation
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
				return fmt.Errorf("manual peripheral operation failed: %w", err)
			}

			return nil
		}
	})
}

func (m *Manager) doHandleAttach() kubeletrc.Attacher {
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

		attachCmd := &aranyagopb.ExecOrAttachCmd{
			Stdin:   stdin != nil,
			Stdout:  stdout != nil,
			Stderr:  stderr != nil,
			Tty:     tty,
			Command: nil,
			Envs:    nil, // currently kubectl attach has no support for environment variables
		}

		if uid != "" {
			// is runtime pod attach

			attachCmd.PodUid = string(uid)
			attachCmd.Container = container

			data, err := attachCmd.Marshal()
			if err != nil {
				return fmt.Errorf("failed to marshal pod attach options: %w", err)
			}

			return m.doServeTerminalStream(aranyagopb.CMD_RUNTIME, &runtimepb.Packet{
				Kind:    runtimepb.CMD_EXEC,
				Payload: data,
			}, stdin, stdout, stderr, resize)
		}

		switch container {
		case constant.VirtualContainerNameHost:
			// is host attach
			return m.doServeTerminalStream(aranyagopb.CMD_ATTACH, attachCmd, stdin, stdout, stderr, resize)
		default:
			// is peripheral metrics collection
			err := m.options.CollectDeviceMetrics(container)
			if err != nil {
				return fmt.Errorf("manual device metrics collection failed: %w", err)
			}
			return nil
		}
	})
}

// nolint:gocyclo
func (m *Manager) doServeTerminalStream(
	kind aranyagopb.CmdType,
	initialCmd proto.Marshaler,
	stdin io.Reader,
	stdout, stderr io.Writer,
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
		// close session, best effort
		_, _, err2 := m.ConnectivityManager.PostCmd(
			sid, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{Sid: sid},
		)

		if err2 != nil {
			logger.I("failed to post session close cmd", log.Error(err2))
		}
	}()

	if resizeCh != nil {
		resizeCmdKind := kind
		if resizeCmdKind != aranyagopb.CMD_RUNTIME {
			resizeCmdKind = aranyagopb.CMD_TTY_RESIZE
		}

		go func() {
			for size := range resizeCh {
				resizeCmd := &aranyagopb.TerminalResizeCmd{
					Cols: uint32(size.Width),
					Rows: uint32(size.Height),
				}

				var cmd proto.Marshaler
				switch resizeCmdKind {
				case aranyagopb.CMD_RUNTIME:
					data, err2 := resizeCmd.Marshal()
					if err2 != nil {
						logger.I("failed to marshal tty resize command", log.Error(err2))
						continue
					}

					cmd = &runtimepb.Packet{
						Kind:    runtimepb.CMD_TTY_RESIZE,
						Payload: data,
					}
				case aranyagopb.CMD_TTY_RESIZE:
					cmd = resizeCmd
				}

				_, _, err2 := m.ConnectivityManager.PostCmd(
					sid, resizeCmdKind, cmd,
				)
				if err2 != nil {
					logger.I("failed to post resize cmd", log.Error(err2))
				}
			}
		}()
	}

	closeRead := make(chan struct{})
	if stdin != nil {
		readTimeout := constant.DefaultNonInteractiveStreamReadTimeout
		if resizeCh != nil {
			readTimeout = constant.DefaultInteractiveStreamReadTimeout
		}

		seq := uint64(0)

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
			go r.FallbackReading(closeRead)

			buf := make([]byte, m.ConnectivityManager.MaxPayloadSize())
			for r.WaitForData(closeRead) {
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

		// unwanted message
		select {
		case <-closeRead:
		default:
			close(closeRead)
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

			// write failure, close read routine
			select {
			case <-closeRead:
			default:
				close(closeRead)
			}

			return true
		}
		return false
	}, func(u interface{}) bool {
		logger.I("unknown message type", log.Any("msg", u))

		// unexpected message, close read routine
		select {
		case <-closeRead:
		default:
			close(closeRead)
		}

		return true
	})

	return err
}

func nextSeq(p *uint64) uint64 {
	seq := atomic.LoadUint64(p)
	for !atomic.CompareAndSwapUint64(p, seq, seq+1) {
		seq++
	}

	return seq
}
