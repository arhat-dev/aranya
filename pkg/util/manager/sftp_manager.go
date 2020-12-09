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

package manager

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"arhat.dev/pkg/log"
)

func NewSFTPManager(
	ctx context.Context,
	rootDir string,
	listener net.Listener,
	hostKey ssh.Signer,
	marshaledSSHPubKey []byte,
) *SFTPManager {
	m := &SFTPManager{
		ctx: ctx,
		log: log.Log.WithName("sftp"),

		rootDir:            rootDir,
		marshaledSSHPubKey: marshaledSSHPubKey,

		listener: listener,
	}

	m.config = &ssh.ServerConfig{
		NoClientAuth: false,
		MaxAuthTries: 6,

		PublicKeyCallback: m.handlePublicKeyAuth,
		AuthLogCallback:   m.handleAuthLog,
		ServerVersion:     "SSH-2.0-ARANYA",
	}
	m.config.AddHostKey(hostKey)

	return m
}

type SFTPManager struct {
	ctx context.Context
	log log.Interface

	// TODO: chroot into node specific dir
	rootDir            string
	config             *ssh.ServerConfig
	marshaledSSHPubKey []byte

	listener net.Listener
}

func (m *SFTPManager) Start() error {
	for {
		nConn, err := m.listener.Accept()
		if err != nil {
			m.log.E("failed to accept incoming connection", log.Error(err))
			return err
		}

		go m.serveClient(nConn)
	}
}

type debugWriter struct {
	logger log.Interface
	r      *bufio.Reader
	io.Writer
}

func newDebugWriter(logger log.Interface) *debugWriter {
	r, w := io.Pipe()

	return &debugWriter{
		logger: logger,
		r:      bufio.NewReader(r),
		Writer: w,
	}
}

func (d *debugWriter) start() {
	for {
		line, _, err := d.r.ReadLine()
		if err != nil {
			return
		}

		d.logger.D(string(line))
	}
}

func (m *SFTPManager) serveClient(nConn net.Conn) {
	logger := m.log.WithFields(log.String("addr", nConn.RemoteAddr().String()))
	logger.I("connection accepted")

	// ssh handshake
	conn, chans, reqs, err := ssh.NewServerConn(nConn, m.config)
	if err != nil {
		logger.I("failed to handshake", log.Error(err))
		return
	}
	_ = conn

	logger = logger.WithName(fmt.Sprintf("sftp.%s", conn.User()))
	logger.I("ssh connection established")

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			err := newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			if err != nil {
				logger.I("failed to reject unknown channel")
			}
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			logger.I("failed to accept channel", log.Error(err))
			continue
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				// nolint:gocritic
				switch req.Type {
				case "subsystem":
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				err := req.Reply(ok, nil)
				if err != nil {
					logger.I("failed to reply to ssh client")
				}
			}
		}(requests)

		var options []sftp.ServerOption
		if logger.Enabled(log.LevelDebug) {
			wr := newDebugWriter(logger)
			go wr.start()
			options = append(options, sftp.WithDebug(wr))
		}

		server, _ := sftp.NewServer(channel, options...)

		func() {
			defer func() { _ = server.Close() }()
			err := server.Serve()

			if err != nil {
				if errors.Is(err, io.EOF) {
					logger.I("sftp client session finished")
				} else {
					logger.I("sftp server completed with error", log.Error(err))
				}
			} else {
				logger.D("sftp client exited no error")
			}
		}()
	}
}

func (m *SFTPManager) handlePublicKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	if !bytes.Equal(key.Marshal(), m.marshaledSSHPubKey) {
		return nil, fmt.Errorf("unknown public key for user %s", conn.User())
	}

	return &ssh.Permissions{
		CriticalOptions: map[string]string{},
		Extensions:      map[string]string{},
	}, nil
}

func (m *SFTPManager) handleAuthLog(conn ssh.ConnMetadata, method string, err error) {
	if method == "none" {
		return
	}

	logger := m.log.WithName(fmt.Sprintf("sftp.%s", conn.User()))
	if err != nil {
		logger.I("authentication failed", log.Error(err))
	} else {
		logger.I("authentication success")
	}
}
