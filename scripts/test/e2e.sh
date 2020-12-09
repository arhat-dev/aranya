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

set -ex

_start_e2e_tests() {
  kube_version=${1}

  rm -rf build/e2e/charts || true
  mkdir -p build/e2e/charts/aranya

  cp -r cicd/deploy/charts/aranya build/e2e/charts/aranya/master

  helm-stack -c e2e/helm-stack.yaml ensure

  # override default values
  cp e2e/values/aranya.yaml "build/e2e/clusters/${kube_version}/default.aranya[aranya@master].yaml"
  cp e2e/values/emqx.yaml "build/e2e/clusters/${kube_version}/emqx.emqx[emqx@v4.2.3].yaml"

  helm-stack -c e2e/helm-stack.yaml gen "${kube_version}"

  export KUBECONFIG="build/e2e/clusters/${kube_version}/kubeconfig.yaml"

  # delete cluster in the end (best effort)
  trap 'kind -v 100 delete cluster --name "${kube_version}" --kubeconfig "${KUBECONFIG}" || true' EXIT

  kind -v 100 create cluster \
    --config "e2e/kind/${kube_version}.yaml" \
    --retain --wait 5m \
    --name "${kube_version}" \
    --kubeconfig "${KUBECONFIG}"

  kubectl get nodes -o yaml
  kubectl taint nodes --all node-role.kubernetes.io/master- || true

  # crd resources may fail at the first time
  helm-stack -c e2e/helm-stack.yaml apply "${kube_version}" || true
  sleep 1
  helm-stack -c e2e/helm-stack.yaml apply "${kube_version}"

  # e2e test time limit
  go test -mod=vendor -v -failfast -race \
    -covermode=atomic -coverprofile="coverage.e2e.${kube_version}.txt" -coverpkg=./... \
    ./e2e/tests/...
}

kube_version="$1"

_start_e2e_tests "${kube_version}"
