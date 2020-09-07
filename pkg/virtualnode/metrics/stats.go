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
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type statsRequest struct {
	// The name of the container for which to request stats.
	// Default: /
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// Max number of stats to return.
	// If start and end time are specified this limit is ignored.
	// Default: 60
	// +optional
	NumStats int `json:"num_stats,omitempty"`

	// Start time for which to query information.
	// If omitted, the beginning of time is assumed.
	// +optional
	Start time.Time `json:"start,omitempty"`

	// End time for which to query information.
	// If omitted, current time is assumed.
	// +optional
	End time.Time `json:"end,omitempty"`

	// Whether to also include information from subcontainers.
	// Default: false.
	// +optional
	Subcontainers bool `json:"subcontainers,omitempty"`
}

func parseStatsRequest(req *http.Request) (statsRequest, error) {
	// Default request.
	query := statsRequest{
		NumStats: 60,
	}

	err := json.NewDecoder(req.Body).Decode(&query)
	if err != nil && err != io.EOF {
		return query, err
	}
	return query, nil
}
