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

package metrics

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	cadvisorapi "github.com/google/cadvisor/info/v1"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/expfmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/util/cache"
)

var gzipWriters = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

func (m *Manager) HandlePprof(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "method not implemented", http.StatusNotImplemented)

	name := strings.TrimPrefix(r.URL.Path, "/debug/pprof")
	switch name {
	case "profile":
	case "symbol":
	case "cmdline":
	case "trace":
	default:
	}
}

// HandleStatsSpec
func (m *Manager) HandleStatsSpec(w http.ResponseWriter, r *http.Request) {
	logger := m.Log.WithFields(log.String("type", "http"), log.String("action", "/spec"))

	info := &cadvisorapi.MachineInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		http.Error(w, "failed to marshal machine info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		logger.I("failed to write data", log.Error(err))
	}
}

// handle root container stats
func (m *Manager) HandleStats(w http.ResponseWriter, r *http.Request) {
	query, err := parseStatsRequest(r)
	if err != nil {
		http.Error(w, "failed to parse request", http.StatusBadRequest)
		return
	}
	_ = query

}

func (m *Manager) HandleStatsSummary(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse request form", http.StatusBadRequest)
		return
	}

	onlyCPUAndMemory := false
	if onlyCluAndMemoryParam, found := r.Form["only_cpu_and_memory"]; found &&
		len(onlyCluAndMemoryParam) == 1 && onlyCluAndMemoryParam[0] == "true" {
		onlyCPUAndMemory = true
	}

	// if onlyCPUAndMemory {

	// } else {

	// }
	_ = onlyCPUAndMemory

	http.Error(w, "method not implemented", http.StatusNotImplemented)
}

func (m *Manager) HandleStatsSystemContainer(w http.ResponseWriter, r *http.Request) {
	query, err := parseStatsRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Non-Kubernetes container stats.
	containerName := path.Join("/", query.ContainerName)
	_ = containerName
	http.Error(w, "method not implemented", http.StatusNotImplemented)
}

func (m *Manager) HandleStatsContainer(w http.ResponseWriter, r *http.Request) {
	query, err := parseStatsRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = query

	// Default parameters.
	params := map[string]string{
		"namespace": metav1.NamespaceDefault,
		"uid":       "",
	}

	for k, v := range mux.Vars(r) {
		params[k] = v
	}

	if params["name"] == "" || params["container"] == "" {
		http.Error(w, fmt.Sprintf("Invalid pod container request: %v", params), http.StatusBadRequest)
		return
	}
	http.Error(w, "method not implemented", http.StatusNotImplemented)
}

func (m *Manager) HandleProbesMetrics(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "method not implemented", http.StatusNotImplemented)
}

func (m *Manager) HandleResourceMetrics(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "method not implemented", http.StatusNotImplemented)
}

// HandleNodeMetrics handle requests to collect metrics exported by node-exporter
func (m *Manager) HandleNodeMetrics(w http.ResponseWriter, r *http.Request) {
	m.serveMetrics(m.nodeMetricsCache, w, r)
}

// HandleContainerMetrics handle requests to collect metrics exported by cAdvisor
func (m *Manager) HandleContainerMetrics(w http.ResponseWriter, r *http.Request) {
	m.serveMetrics(m.containerMetricsCache, w, r)
}

func (m *Manager) serveMetrics(cache *cache.MetricsCache, resp http.ResponseWriter, r *http.Request) {
	select {
	case <-m.Context().Done():
		http.Error(resp, "server exited", http.StatusInternalServerError)
		return
	default:
	}

	switch cache {
	case m.nodeMetricsCache:
		if !m.nodeMetricsSupported() {
			http.Error(resp, "node metrics not supported", http.StatusNotImplemented)
			return
		}

		m.retrieveDeviceMetrics(&aranyagopb.MetricsCollectCmd{Target: aranyagopb.METRICS_TARGET_NODE})
	case m.containerMetricsCache:
		if !m.options.ContainerMetrics.Enabled || !m.containerMetricsSupported() {
			http.Error(resp, "container metrics not supported", http.StatusNotImplemented)
			return
		}

		m.retrieveDeviceMetrics(&aranyagopb.MetricsCollectCmd{Target: aranyagopb.METRICS_TARGET_CONTAINER})
	}

	contentType := expfmt.Negotiate(r.Header)
	header := resp.Header()
	header.Set("Content-Type", string(contentType))

	w := io.Writer(resp)
	if gzipAccepted(r.Header) {
		header.Set("Content-Encoding", "gzip")
		gz := gzipWriters.Get().(*gzip.Writer)
		defer gzipWriters.Put(gz)

		gz.Reset(w)
		defer func() { _ = gz.Close() }()

		w = gz
	}

	enc := expfmt.NewEncoder(w, contentType)

	var lastErr error
	for _, mf := range cache.Get() {
		if err := enc.Encode(mf); err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		header.Del("Content-Encoding")
		http.Error(resp,
			"An error has occurred while serving metrics:\n\n"+lastErr.Error(),
			http.StatusInternalServerError,
		)
	}
}

// gzipAccepted returns whether the client will accept gzip-encoded content.
func gzipAccepted(header http.Header) bool {
	a := header.Get("Accept-Encoding")
	parts := strings.Split(a, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "gzip" || strings.HasPrefix(part, "gzip;") {
			return true
		}
	}
	return false
}
