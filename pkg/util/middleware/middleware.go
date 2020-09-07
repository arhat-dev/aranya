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

package middleware

import (
	"net/http"
	"time"

	"arhat.dev/pkg/log"
)

func NotFoundHandler(logger log.Interface) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.V("request url not handled", log.String("uri", r.URL.RequestURI()))
		http.Error(w, "404 page not found", http.StatusNotFound)
	})
}

func DeviceOnlineCheck(
	checkDisconnected func() <-chan struct{},
	baseLogger log.Interface) func(http.Handler,
) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := baseLogger.WithFields(log.String("uri", r.URL.RequestURI()))
			startTime := time.Now()

			logger.V("request started")

			defer func() {
				err := recover()
				if err != nil {
					logger.E("recovered from panic", log.Any("panic", err))
				}

				logger.V("request finished", log.Duration("dur", time.Since(startTime)))
			}()

			select {
			case <-checkDisconnected():
				http.Error(w, "device not connected", http.StatusServiceUnavailable)
				return
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
