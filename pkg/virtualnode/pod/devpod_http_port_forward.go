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
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
	"arhat.dev/pkg/log"
	"github.com/gogo/protobuf/proto"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apiserver/pkg/util/wsstream"
	api "k8s.io/kubernetes/pkg/apis/core"
	kubeletpf "k8s.io/kubernetes/pkg/kubelet/cri/streaming/portforward"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

const (
	v4BinaryWebsocketProtocol = "v4." + wsstream.ChannelWebSocketProtocol
	v4Base64WebsocketProtocol = "v4." + wsstream.Base64ChannelWebSocketProtocol
)

func (m *Manager) servePortForward(
	logger log.Interface,
	w http.ResponseWriter,
	r *http.Request,
	podUID string,
	wsPfOpts *kubeletpf.V4Options, // for websocket stream only
	idleTimeout, streamCreationTimeout time.Duration,
	supportedProtocols []string,
) error {
	if wsstream.IsWebSocketRequest(r) {
		logger.V("serving websocket")
		return m.handleWebSocketPortForward(logger, w, r, podUID, idleTimeout, wsPfOpts)
	}

	logger.V("serving spdy streams")
	return m.handleHTTPPortForward(logger, w, r, podUID, idleTimeout, streamCreationTimeout, supportedProtocols)
}

func (m *Manager) handleWebSocketPortForward(
	logger log.Interface,
	w http.ResponseWriter,
	r *http.Request,
	podUID string,
	idleTimeout time.Duration,
	opts *kubeletpf.V4Options,
) error {
	channels := make([]wsstream.ChannelType, 0, len(opts.Ports)*2)
	for i := 0; i < len(opts.Ports); i++ {
		channels = append(channels, wsstream.ReadWriteChannel, wsstream.WriteChannel)
	}
	conn := wsstream.NewConn(map[string]wsstream.ChannelProtocolConfig{
		"": {
			Binary:   true,
			Channels: channels,
		},
		v4BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
		v4Base64WebsocketProtocol: {
			Binary:   false,
			Channels: channels,
		},
	})
	conn.SetIdleTimeout(idleTimeout)

	_, streams, err := conn.Open(w, r)
	if err != nil {
		err = fmt.Errorf("failed to upgrade websocket connection: %w", err)
		return err
	}

	wg := new(sync.WaitGroup)
	defer func() {
		wg.Wait()
		_ = conn.Close()
	}()

	for i := range opts.Ports {
		s := &stream{
			podUID:  podUID,
			network: "tcp",
			port:    opts.Ports[i],

			dataStream:  streams[i*2],
			errorStream: streams[i*2+1],
		}

		portBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(portBytes, uint16(s.port))
		_, _ = s.dataStream.Write(portBytes)
		_, _ = s.errorStream.Write(portBytes)

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()

			m.doPortForward(r.Context(), logger, wg, s)
		}()
	}

	return nil
}

func (m *Manager) handleHTTPPortForward(
	logger log.Interface,
	w http.ResponseWriter,
	r *http.Request,
	podUID string,
	idleTimeout, streamCreationTimeout time.Duration,
	supportedProtocols []string,
) error {
	_, err := httpstream.Handshake(r, w, supportedProtocols)
	// negotiated protocol isn't currently used server side, but could be in the future
	if err != nil {
		// Handshake writes the error to the client
		return err
	}
	streamChan := make(chan httpstream.Stream, 16)

	up := spdy.NewResponseUpgrader()
	conn := up.UpgradeResponse(w, r, validateNewHTTPStream(streamChan))
	if conn == nil {
		return fmt.Errorf("failed to upgrade http stream connection")
	}
	conn.SetIdleTimeout(idleTimeout)

	wg := new(sync.WaitGroup)
	timeoutCheck := time.NewTimer(streamCreationTimeout / 5)

	streamPairs := make(map[string]*stream)
	for {
		select {
		case <-conn.CloseChan():
			logger.D("request closed, closing all streams")
			if !timeoutCheck.Stop() {
				<-timeoutCheck.C
			}

			for _, s := range streamPairs {
				s.close("")
			}

			_ = conn.Close()

			wg.Wait()

			logger.V("all streams exited")

			return nil
		case <-timeoutCheck.C:
			logger.V("detecting expired streams")
			timeoutCheck.Reset(streamCreationTimeout / 5)

			var toDelete []string
			now := time.Now()
			for reqID, p := range streamPairs {
				// stream pair created, check if its creation has timed out
				if now.After(p.creationFailAt) && !p.prepared() {
					logger.D("stream creation timeout", log.String("reqID", reqID))

					p.close("stream creation timeout")
					toDelete = append(toDelete, reqID)
				}
			}

			for _, id := range toDelete {
				delete(streamPairs, id)
			}
		case hs := <-streamChan:
			reqID := hs.Headers().Get(api.PortForwardRequestIDHeader)
			logger.V("new stream created", log.String("reqID", reqID))

			s, hasStreamPair := streamPairs[reqID]
			if !hasStreamPair {
				s = &stream{
					reqID: reqID,

					creationFailAt: time.Now().Add(streamCreationTimeout),
					podUID:         podUID,
					network:        "tcp",
				}
				streamPairs[reqID] = s
			}

			err = m.handleNewHTTPStream(r.Context(), logger.WithFields(log.String("reqID", s.reqID)), wg, s, hs)
			if err != nil {
				logger.D("invalid stream", log.Error(err))
				s.writeErr(err.Error())
			}
		}
	}
}

func (m *Manager) handleNewHTTPStream(
	ctx context.Context,
	logger log.Interface,
	wg *sync.WaitGroup,
	s *stream,
	hs httpstream.Stream,
) error {
	switch t := hs.Headers().Get(api.StreamType); t {
	case api.StreamTypeData:
		if s.dataStream != nil {
			return fmt.Errorf("data stream already assigned")
		}

		logger.V("set data stream")

		portString := hs.Headers().Get(api.PortHeader)
		port, _ := strconv.ParseInt(portString, 10, 32)

		s.dataStream = hs
		s.port = int32(port)
	case api.StreamTypeError:
		if s.errorStream != nil {
			return fmt.Errorf("error stream already assigned")
		}

		logger.V("set error stream")

		s.errorStream = hs
	default:
		logger.D("ignored stream", log.String("streamType", t))
		return nil
	}

	if !s.prepared() {
		return nil
	}

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()

			logger.D("stream finished")
			s.close("")
		}()

		m.doPortForward(ctx, logger, wg, s)
	}()

	return nil
}

func (m *Manager) doPortForward(ctx context.Context, logger log.Interface, wg *sync.WaitGroup, s *stream) {
	_ = ctx

	logger.D("forwarding to remote")

	pfCmd := &aranyagopb.PortForwardCmd{
		PodUid:  s.podUID,
		Network: s.network,
		Address: s.address,
		Port:    s.port,
	}

	var (
		kind aranyagopb.CmdType
		cmd  proto.Marshaler
	)

	if s.podUID == "" {
		// is host port-forward
		kind = aranyagopb.CMD_PORT_FORWARD
		cmd = pfCmd
	} else {
		// is runtime pod port-forward
		data, err := pfCmd.Marshal()
		if err != nil {
			logger.D("failed to marshal port-forward options", log.Error(err))
			s.close(fmt.Sprintf("prepare: %v", err))
			return
		}

		kind = aranyagopb.CMD_RUNTIME
		cmd = &runtimepb.Packet{
			Kind:    runtimepb.CMD_PORT_FORWARD,
			Payload: data,
		}
	}

	msgCh, streamReady, sid, err := m.ConnectivityManager.PostStreamCmd(
		kind, cmd, s.dataStream, s.errorStream, false, nil,
	)
	if err != nil {
		logger.D("failed to create session", log.Error(err))
		s.close(fmt.Sprintf("prepare: %v", err))
		return
	}

	// write routine is closed when read routine finished

	if wg != nil {
		wg.Add(1)
	}
	go func() {
		defer func() {
			s.close("")

			if wg != nil {
				wg.Done()
			}
		}()

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("error happened", log.Error(msgErr))
				s.close(fmt.Sprintf("error happened in remote: %s", msgErr.Error()))
			} else {
				logger.I("unexpected non data msg", log.Int32("kind", int32(msg.Kind)), log.Uint64("sid", sid))
				s.close(fmt.Sprintf("unexpected non data msg %d", msg.Kind))
			}

			return true
		})
	}()

	// read routine

	var seq uint64
	defer func() {
		_, _, _, err = m.ConnectivityManager.PostData(sid, aranyagopb.CMD_DATA_UPSTREAM, nextSeq(&seq), true, nil)
		if err != nil {
			logger.D("failed to post read close cmd", log.Error(err))
		}

		// close session with best effort
		_, _, _ = m.ConnectivityManager.PostCmd(sid, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{
			Sid: sid,
		})
	}()

	bufSize := m.ConnectivityManager.MaxPayloadSize()
	if bufSize > constant.MaxBufSize {
		bufSize = constant.MaxBufSize
	}

	// wait until stream prepared
	select {
	case <-ctx.Done():
		return
	case <-streamReady:
	}

	var n int
	buf := make([]byte, bufSize)

	for {
		n, err = s.dataStream.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.I("failed to read data", log.Error(err))
				s.writeErr(fmt.Sprintf("read: %v", err))
			}

			if n == 0 {
				return
			}
		}

		data := make([]byte, n)
		_ = copy(data, buf)

		// do not check returned last seq since we have limited the buffer size
		_, _, _, err = m.ConnectivityManager.PostData(
			sid, aranyagopb.CMD_DATA_UPSTREAM, nextSeq(&seq), false, data,
		)

		if err != nil {
			// TODO: shall we redo data post until successful?
			logger.I("failed to post data", log.Error(err))
			s.writeErr(fmt.Sprintf("post: %v", err))
			return
		}
	}
}

type stream struct {
	reqID string

	creationFailAt time.Time

	podUID  string
	network string
	address string
	port    int32

	dataStream  io.ReadWriteCloser
	errorStream io.WriteCloser
}

func (s *stream) prepared() bool {
	// s.podUID can be empty (for host port-forward)
	// s.network can be empty (default to tcp)
	return s.dataStream != nil && s.errorStream != nil && s.port > 0
}

func (s *stream) close(reason string) {
	if reason != "" {
		s.writeErr(fmt.Sprintf("[%s]: %s", s.reqID, reason))
	}

	if s.dataStream != nil {
		_ = s.dataStream.Close()
	}

	if s.errorStream != nil {
		_ = s.errorStream.Close()
	}
}

func (s *stream) writeErr(err string) {
	if s.errorStream != nil {
		_, _ = s.errorStream.Write([]byte(err))
	}
}

// validateNewHTTPStream is the httpstream.NewStreamHandler for port
// forward streams. It checks each stream's port and stream type headers,
// rejecting any streams that with missing or invalid values. Each valid
// stream is sent to the streams channel.
func validateNewHTTPStream(streams chan<- httpstream.Stream) func(httpstream.Stream, <-chan struct{}) error {
	return func(stream httpstream.Stream, replySent <-chan struct{}) error {
		// make sure it has a valid port header
		portString := stream.Headers().Get(api.PortHeader)
		if len(portString) == 0 {
			return fmt.Errorf("%q header is required", api.PortHeader)
		}
		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return fmt.Errorf("unable to parse %q as a port: %v", portString, err)
		}
		if port < 1 {
			return fmt.Errorf("port %q must be > 0", portString)
		}

		// make sure it has a valid stream type header
		streamType := stream.Headers().Get(api.StreamType)
		if len(streamType) == 0 {
			return fmt.Errorf("%q header is required", api.StreamType)
		}
		if streamType != api.StreamTypeError && streamType != api.StreamTypeData {
			return fmt.Errorf("invalid stream type %q", streamType)
		}

		streams <- stream
		return nil
	}
}

func (m *Manager) doCustomPortForward(w http.ResponseWriter, r *http.Request, logger log.Interface) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	opts := new(aranyagoconst.CustomPortForwardOptions)
	err := dec.Decode(opts)
	if err != nil {
		logger.I("failed to resolve custom port-forward options", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.I("response is not a hijacker")
		http.Error(w, "unable to hijack response", http.StatusInternalServerError)
		return
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		logger.I("failed to hijack response", log.Error(err))
		http.Error(w, "unable to hijack response", http.StatusInternalServerError)
		return
	}

	// clear read/write deadline
	_ = conn.SetReadDeadline(time.Time{})
	_ = conn.SetWriteDeadline(time.Time{})

	// create a session for port-forward stream now

	canWrite := make(chan struct{})
	msgCh, streamReady, sid, err := m.ConnectivityManager.PostStreamCmd(
		aranyagopb.CMD_PORT_FORWARD,
		&aranyagopb.PortForwardCmd{
			PodUid:  "", // always empty since issued for host operation
			Network: opts.Network,
			Address: opts.Address,
			Port:    opts.Port,
		},
		conn, nil, opts.Packet, canWrite,
	)
	if err != nil {
		close(canWrite)
		_ = conn.Close()

		logger.I("failed to create session", log.Error(err))
		return
	}

	respHeader := make(http.Header)
	respHeader.Set(aranyagoconst.HeaderSessionID,
		strconv.FormatInt(int64(sid), 10),
	)
	respHeader.Set(aranyagoconst.HeaderMaxPayloadSize,
		strconv.FormatInt(int64(m.ConnectivityManager.MaxPayloadSize()), 10),
	)

	resp := &http.Response{
		StatusCode: http.StatusSwitchingProtocols,
		Header:     respHeader,
		Close:      false,
	}

	err = resp.Write(conn)
	if err != nil {
		close(canWrite)
		_ = conn.Close()

		logger.I("failed to write header response", log.Error(err))
		return
	}

	close(canWrite)

	// we are good to go, handle port-forward stream now

	go func() {
		// do not close connection until session closed in case data remain unread

		defer func() {
			_ = conn.Close()
		}()

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("error happened", log.Error(msgErr))
			} else {
				logger.I("unexpected non data msg", log.Int32("kind", int32(msg.Kind)), log.Uint64("sid", sid))
			}

			return true
		})
	}()

	// handle read routine

	var seq uint64
	defer func() {
		_, _, _, err = m.ConnectivityManager.PostData(sid, aranyagopb.CMD_DATA_UPSTREAM, nextSeq(&seq), true, nil)
		if err != nil {
			logger.D("failed to post read close cmd", log.Error(err))
		}

		// close session with best effort
		_, _, _ = m.ConnectivityManager.PostCmd(sid, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{
			Sid: sid,
		})
	}()

	// wait until stream prepared
	select {
	case <-r.Context().Done():
		return
	case <-streamReady:
	}

	// only ensure data sent when forwarded target is stream oriented
	ensureSend := !opts.Packet

	maxSize := uint64(m.ConnectivityManager.MaxPayloadSize())
	br := bufio.NewReader(conn)

	// cleanup unread data in conn to start
	tr := textproto.NewReader(br)
	ok = false
	for i := 0; i < 5; i++ {
		var line string
		line, err = tr.ReadLine()
		if err != nil {
			logger.I("failed to prepare for read routine", log.Error(err))
			return
		}

		if line == "port-forward" {
			ok = true
			break
		}
	}
	if !ok {
		logger.I("unexpected start message not found", log.Error(err))
		return
	}

	for {
		size, err2 := binary.ReadUvarint(br)
		if err2 != nil {
			logger.I("failed to read packet size", log.Error(err2))
			return
		}

		if size > maxSize {
			// invalid packet length, client didn't follow max payload size limit
			logger.I("invalid too large data chunk",
				log.Uint64("size", size),
				log.Uint64("max_size", maxSize),
			)
			return
		}

		data := make([]byte, size)
		_, err2 = io.ReadFull(br, data)
		if err2 != nil {
			logger.I("failed to read packet data", log.Error(err2))
			return
		}

		err2 = m.ConnectivityManager.PostEncodedData(data, ensureSend)
		if err2 != nil {
			logger.I("failed to post data", log.Error(err2))
			if ensureSend {
				// only can happen when this virtual node is exiting
				return
			}
		}
	}
}
