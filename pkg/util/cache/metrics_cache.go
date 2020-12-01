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

package cache

import (
	"bytes"
	"sync"

	"github.com/klauspost/compress/zstd"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var zstdDecoder *zstd.Decoder

func init() {
	var err error
	zstdDecoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(err)
	}
}

func NewMetricsCache() *MetricsCache {
	return new(MetricsCache)
}

type MetricsCache struct {
	mfs []*dto.MetricFamily
	mu  sync.Mutex
}

func (c *MetricsCache) Update(metricsBytes []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	mfs, err := parseMetricsBytes(metricsBytes)
	if err != nil {
		return err
	}

	if len(mfs) == 0 {
		c.mfs = mfs
	} else {
		c.mfs = append(c.mfs, mfs...)
	}

	return nil
}

func (c *MetricsCache) Get() []*dto.MetricFamily {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := c.mfs
	c.mfs = nil
	return result
}

func parseMetricsBytes(metricsBytes []byte) ([]*dto.MetricFamily, error) {
	data, err := zstdDecoder.DecodeAll(metricsBytes, nil)
	if err != nil {
		return nil, err
	}

	decoder := expfmt.NewDecoder(bytes.NewReader(data), expfmt.FmtProtoDelim)

	var mfs []*dto.MetricFamily
	for {
		mf := new(dto.MetricFamily)
		err = decoder.Decode(mf)
		if err != nil {
			break
		}
		mfs = append(mfs, mf)
	}

	return mfs, nil
}
