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
