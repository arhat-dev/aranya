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

_create_edge_devices() {
  kind_cluster_name=${1}

  cat <<EOF | kubectl apply -f -
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: e2e-alice
  namespace: default
spec:
  connectivity:
    method: mqtt
    mqtt:
      broker: emqx.edge:1883
      clientID: aranya.e2e(${kind_cluster_name}-worker)
      topicNamespace: e2e.aranya.arhat.dev/${kind_cluster_name}-worker
      transport: tcp
---
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: e2e-bob
  namespace: default
spec:
  connectivity:
    method: mqtt
    mqtt:
      broker: emqx.edge:1883
      clientID: aranya.e2e(${kind_cluster_name}-worker2)
      topicNamespace: e2e.aranya.arhat.dev/${kind_cluster_name}-worker2
      transport: tcp
---
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: e2e-foo
  namespace: full
spec:
  connectivity:
    method: mqtt
    mqtt:
      broker: emqx.edge:1883
      clientID: aranya.e2e(${kind_cluster_name}-worker3)
      topicNamespace: e2e.aranya.arhat.dev/${kind_cluster_name}-worker3
      transport: tcp
---
apiVersion: aranya.arhat.dev/v1alpha1
kind: EdgeDevice
metadata:
  name: e2e-bar
  namespace: full
spec:
  connectivity:
    method: mqtt
    mqtt:
      broker: emqx.edge:1883
      clientID: aranya.e2e(${kind_cluster_name}-worker4)
      topicNamespace: e2e.aranya.arhat.dev/${kind_cluster_name}-worker4
      transport: tcp
EOF
}

_start_e2e_tests() {
  kube_version=${1}

  rm -rf build/e2e/charts || true
  mkdir -p build/e2e/charts/aranya

  # copy local charts to chart dir
  cp -r cicd/deploy/charts/aranya build/e2e/charts/aranya/master

  KUBECONFIG="${ARANYA_E2E_KUBECONFIG}" ${helm_stack} ensure

  # override default values
  cp e2e/values/aranya.yaml "build/e2e/clusters/${kube_version}/default.aranya[aranya@master].yaml"
  cp e2e/values/aranya-full.yaml "build/e2e/clusters/${kube_version}/full.aranya[aranya@master].yaml"
  cp e2e/values/emqx.yaml "build/e2e/clusters/${kube_version}/emqx.emqx[emqx@v4.2.3].yaml"
  cp e2e/values/arhat.yaml "build/e2e/clusters/${kube_version}/remote.arhat[arhat-dev.arhat@latest].yaml"

  KUBECONFIG="${ARANYA_E2E_KUBECONFIG}" ${helm_stack} gen "${kube_version}"

  # delete cluster in the end (best effort)
  # trap '${kind} delete cluster --name "${kube_version}" || true' EXIT

  ${kind} create cluster --name "${kube_version}" \
    --config "e2e/kind/${kube_version}.yaml" \
    --retain --wait 5m

  # ensure tenant namespace
  ${kubectl} create namespace tenant

  # crd resources may fail at the first time
  KUBECONFIG="${ARANYA_E2E_KUBECONFIG}" ${helm_stack} apply "${kube_version}" || true
  sleep 1
  KUBECONFIG="${ARANYA_E2E_KUBECONFIG}" ${helm_stack} apply "${kube_version}"

  # wait until aranya running
  while ! ${kubectl} get po --namespace default | grep aranya | grep Running ; do
    echo "waiting for aranya running in namespace 'default'"
    sleep 1
  done

  echo "aranya running in namespace 'default'"

  while ! ${kubectl} get po --namespace full | grep aranya | grep Running ; do
    echo "waiting for aranya running in namespace 'full'"
    sleep 1
  done

  echo "aranya running in namespace 'full'"

  # create edge devices after aranya is running
  _create_edge_devices "${kube_version}"

  # give aranya 30s to create related resources
  sleep 30

  # should be able to find new virtual nodes now (for debugging)
  ${kubectl} get nodes -o wide
  ${kubectl} get certificatesigningrequests
  ${kubectl} get pods --all-namespaces

  ARANYA_E2E_KUBECONFIG="${ARANYA_E2E_KUBECONFIG}" \
    go test -mod=vendor -v -failfast -race \
    -covermode=atomic -coverprofile="coverage.e2e.${kube_version}.txt" -coverpkg=./... \
    ./e2e/tests/...
}

kube_version="$1"
ARANYA_E2E_KUBECONFIG="${ARANYA_E2E_KUBECONFIG:-$(mktemp)}"
echo "using kubeconfig '${ARANYA_E2E_KUBECONFIG}' for e2e"

helm_stack="helm-stack -c e2e/helm-stack"
kind="kind -v 100 --kubeconfig '${ARANYA_E2E_KUBECONFIG}'"
kubectl="kubectl --kubeconfig '${ARANYA_E2E_KUBECONFIG}'"

_start_e2e_tests "${kube_version}"
