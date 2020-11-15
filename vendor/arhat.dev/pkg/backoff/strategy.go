/*
Copyright 2019 The arhat.dev Authors.

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

package backoff

import (
	"sync"
	"time"
)

// NewStrategy returns a backoff calculation strategy
func NewStrategy(initialDelay, maxDelay time.Duration, factor float64, backoffThreshold uint64) *Strategy {
	return &Strategy{
		initialDelay:     initialDelay,
		maxDelay:         maxDelay,
		factor:           factor,
		backoffThreshold: backoffThreshold,

		m:  make(map[interface{}]backoffItem),
		mu: new(sync.Mutex),
	}
}

type backoffItem struct {
	currentDelay time.Duration
	counter      uint64
}

// Strategy defines how to calculate backoff time
type Strategy struct {
	initialDelay     time.Duration
	maxDelay         time.Duration
	factor           float64
	backoffThreshold uint64

	m  map[interface{}]backoffItem
	mu *sync.Mutex
}

// Next returns next backoff time of key
func (b *Strategy) Next(key interface{}) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentBackoff, ok := b.m[key]
	if !ok {
		currentBackoff = backoffItem{}
	}

	counter := currentBackoff.counter
	var nextDelay time.Duration
	switch {
	case counter == b.backoffThreshold:
		nextDelay = b.initialDelay
	case counter > b.backoffThreshold:
		nextDelay = time.Duration(b.factor * float64(currentBackoff.currentDelay))

		if nextDelay > b.maxDelay {
			nextDelay = b.maxDelay
		}
	default:
		nextDelay = 0
	}

	b.m[key] = backoffItem{currentDelay: nextDelay, counter: counter + 1}
	return nextDelay
}

// Reset backoff time for key
func (b *Strategy) Reset(key interface{}) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, found := b.m[key]
	if found {
		delete(b.m, key)
		return true
	}
	return false
}
