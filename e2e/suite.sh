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

wait_for_pods() {
  namespace="${1}"
  label_selector="${2}"

  for _ in $(seq 0 1 12); do
    if ! kubectl wait --namespace "${namespace}" --for=condition=Ready \
      --selector "${label_selector}" pods --all --timeout=30s; then
      kubectl get pods \
        --all-namespaces -o wide || true

      kubectl describe pods \
        --namespace "${namespace}" \
        --selector "${label_selector}" || true
    else
      break
    fi
  done
}

log_pods_prev() {
  namespace="${1}"
  label_selector="${2}"
  log_file="${3}"

  kubectl --namespace "${namespace}" logs \
    --previous --prefix --tail=-1 --selector "${label_selector}" \
    >"${log_file}" 2>&1 || true
}

get_aranya_leader_pod_name() {
  namespace="${1}"

  kubectl --namespace "${namespace}" get pods \
    --selector 'aranya.arhat.dev/leadership=leader' \
    -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true
}

log_and_cleanup() {
  kube_version="${1}"

  result_dir="build/e2e/results/${kube_version}"
  mkdir -p "${result_dir}/cluster-dump"

  log_pods_prev default 'app.kubernetes.io/name=aranya' "${result_dir}/aranya-default.prev.log"
  log_pods_prev full 'app.kubernetes.io/name=aranya' "${result_dir}/aranya-full.prev.log"
  log_pods_prev tenant 'app.kubernetes.io/component=abbot' "${result_dir}/abbot-tenant.prev.log"
  log_pods_prev remote 'app.kubernetes.io/name=arhat' "${result_dir}/arhat-remote.prev.log"

  kube_components="etcd kube-apiserver kube-scheduler kube-controller-manager"
  for comp in ${kube_components}; do
    log_pods_prev kube-system "tier=control-plane,component=${comp}" "${result_dir}/${comp}.prev.log"
  done

  kubectl cluster-info dump --all-namespaces --output-directory="${result_dir}/cluster-dump" || true

  aranya_namespaces="default full"
  for ns in ${aranya_namespaces}; do
    leader_pod="$(get_aranya_leader_pod_name "${ns}")"
    # kill aranya process to get coverage profile
    aranya_pid="$(kubectl exec --namespace "${ns}" "${leader_pod}" -- pidof aranya)"
    kubectl exec --namespace "${ns}" "${leader_pod}" -- bash -c "kill -s SIGINT ${aranya_pid}"
    # wait for test profile write
    sleep 20
    # copy aranya test profiles
    kubectl cp "${ns}/${leader_pod}:/profile" "${result_dir}/profile-aranya-${ns}" || true
  done

  if [ "${ARANYA_E2E_CLEAN}" = "1" ]; then
    kind delete cluster --name "${kube_version}" || true
  fi
}

start_e2e_tests() {
  kube_version="${1}"

  rm -rf build/e2e/charts || true
  mkdir -p build/e2e/charts/aranya

  # copy local charts to chart dir
  cp -r cicd/deploy/charts/aranya build/e2e/charts/aranya/master

  helm_stack="helm-stack -c e2e/helm-stack"
  ${helm_stack} ensure

  # override default values
  chart_values_dir="build/e2e/clusters/${kube_version}"
  cp e2e/values/emqx.yaml "${chart_values_dir}/emqx.emqx[emqx@v4.2.3].yaml"
  cp e2e/values/aranya.yaml "${chart_values_dir}/default.aranya[aranya@master].yaml"
  cp e2e/values/aranya-full.yaml "${chart_values_dir}/full.aranya[aranya@master].yaml"
  cp e2e/values/arhat.yaml "${chart_values_dir}/remote.arhat[arhat-dev.arhat@latest].yaml"

  ${helm_stack} gen "${kube_version}"

  # delete cluster in the end (best effort)
  trap 'log_and_cleanup "${kube_version}" || true' EXIT

  # do not set --wait since we are using custom CNI plugins
  kind -v 100 create cluster --name "${kube_version}" \
    --config "e2e/kind/${kube_version}.yaml" \
    --retain --kubeconfig "${KUBECONFIG}"

  docker network disconnect "kind" "kind-registry" || true
  docker network connect "kind" "kind-registry"

  # ensure tenant namespace
  kubectl create namespace sys
  kubectl create namespace tenant

  # crd resources may fail at the first time, do it indefinitely to tolerate
  # api server error
  while ! ${helm_stack} apply "${kube_version}"; do
    sleep 10
  done

  echo "waiting for coredns"
  wait_for_pods kube-system 'k8s-app=kube-dns'

  echo "waiting for aranya running in namespace 'default'"
  wait_for_pods default 'app.kubernetes.io/name=aranya'

  echo "waiting for aranya running in namespace 'full'"
  wait_for_pods full 'app.kubernetes.io/name=aranya'

  echo "waiting for abbot running in namespace 'tenant'"
  wait_for_pods tenant 'app.kubernetes.io/component=abbot'

  echo "waiting for arhat running in namespace 'remote'"
  wait_for_pods remote 'app.kubernetes.io/name=arhat'

  # create edge devices after aranya is running
  device_manifests="alice bob foo bar"
  for m in ${device_manifests}; do
    echo "creating EdgeDevice ${m}"
    kind_cluster_name="${kube_version}"
    export kind_cluster_name

    while ! (
      echo "cat <<EOF"
      cat "e2e/testdata/edgedevices/${m}.yaml"
      echo EOF
    ) | sh | kubectl apply -f -; do
      sleep 10
    done
  done

  # give aranya 60s to create related resources
  sleep 60

  # restart arhat pods to update node ext info
  # NOTE: make sure restart count is equal to arhatRestartCount-1 in tests/node_test.go
  for _ in $(seq 1 1 4); do
    kubectl delete po --namespace remote --all --grace-period 0
    # wait for pod deletion
    sleep 5

    # we may crash kube-controller-manager with our managed nodes, and we should
    # detect that error, so DO NOT wait for kube-controller-manager
    #   wait_for_pods kube-system 'tier=control-plane,component=kube-controller-manager'
    # but its hard to tell whether the error was caused by us (kind cluster error?)

    wait_for_pods remote 'app.kubernetes.io/name=arhat'
    # wait for connection
    sleep 5
  done

  kubectl get nodes -o wide || true

  go test -mod=vendor -v ./e2e/tests/...
}

kube_version="$1"
ARANYA_E2E_CLEAN="${ARANYA_E2E_CLEAN:-"1"}"
ARANYA_E2E_KUBECONFIG="${ARANYA_E2E_KUBECONFIG:-$(mktemp)}"
echo "using kubeconfig '${ARANYA_E2E_KUBECONFIG}' for e2e"

export KUBECONFIG="${ARANYA_E2E_KUBECONFIG}"
export ARANYA_E2E_KUBECONFIG

start_e2e_tests "${kube_version}"
