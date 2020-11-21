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
	"net"
	"net/http"
	pprofweb "net/http/pprof"
	"path/filepath"
	"runtime"
)

func NewPProfManager(listenAddr, basePath string, mutexProfileFraction, blockProfileRate int) (*PProfManager, error) {
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	runtime.SetMutexProfileFraction(mutexProfileFraction)
	runtime.SetBlockProfileRate(blockProfileRate)

	return &PProfManager{
		listener: l,
		basePath: basePath,
	}, nil
}

type PProfManager struct {
	listener net.Listener
	basePath string
}

func (m *PProfManager) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc(m.basePath, pprofweb.Index)
	mux.HandleFunc(filepath.Join(m.basePath, "cmdline"), pprofweb.Cmdline)
	mux.HandleFunc(filepath.Join(m.basePath, "profile"), pprofweb.Profile)
	mux.HandleFunc(filepath.Join(m.basePath, "symbol"), pprofweb.Symbol)
	mux.HandleFunc(filepath.Join(m.basePath, "trace"), pprofweb.Trace)

	pprofSrv := &http.Server{Handler: mux}

	return pprofSrv.Serve(m.listener)
}
