#!/bin/sh

# Copyright 2020 The arhat.dev Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

common_go_test_env="GOOS=$(go env GOHOSTOS) GOARCH=$(go env GOHOSTARCH)"
common_go_test_flags="-mod=vendor -v -failfast -covermode=atomic"

pkg() {
    go_test="${common_go_test_env} CGO_ENABLED=1 go test ${common_go_test_flags} -race -coverprofile=coverage.pkg.txt -coverpkg=./pkg/... ./pkg/..."

    set -ex
    eval "${go_test}"
}

cmd() {
    go_test="${common_go_test_env} CGO_ENABLED=0 go test ${common_go_test_flags} -coverprofile=coverage.cmd.txt -coverpkg=./cmd/... ./cmd/..."

    set -ex
    eval "${go_test}"
}

$1
