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

e2e() {
  kube_version=${1}

  rm -rf build/e2e/charts || true

  mkdir -p build/e2e/charts/template-kubernetes-controller

  cp -r cicd/deploy/charts/template-kubernetes-controller \
    build/e2e/charts/template-kubernetes-controller/master

  helm-stack -c e2e/helm-stack.yaml ensure

  # basic override for image pull secrets
  cp e2e/values/template-kubernetes-controller.yaml "build/e2e/clusters/${kube_version}/template-kubernetes-controller_master.default_template-kubernetes-controller.yaml"
  cp e2e/values/emqx.yaml "build/e2e/clusters/${kube_version}/emqx_v4.1.1.emqx_emqx.yaml"

  helm-stack -c e2e/helm-stack.yaml gen "${kube_version}"

  export KUBECONFIG="build/e2e/clusters/${kube_version}/kubeconfig.yaml"

  # delete cluster (best effort)
  trap 'kind -v 100 delete cluster --name "${kube_version}" --kubeconfig "${KUBECONFIG}" || true ' EXIT

	kind -v 100 create cluster --config "e2e/kind/${kube_version}.yaml" --retain --wait 5m --name "${kube_version}" --kubeconfig "${KUBECONFIG}"

  kubectl get nodes -o yaml
  kubectl taint nodes --all node-role.kubernetes.io/master- || true
  kubectl apply -f e2e/manifests

  # crd resources may fail at first time
  helm-stack -c e2e/helm-stack.yaml apply "${kube_version}" || true
  sleep 1
  helm-stack -c e2e/helm-stack.yaml apply "${kube_version}"

  set +ex

  if [ -n "${E2E_IMAGE_REGISTRY_USER}" ]; then
    for ns in $(kubectl get ns -o jsonpath='{.items[*].metadata.name}'); do
      kubectl create -n "${ns}" secret \
        docker-registry \
        docker-pull-secret \
        --docker-username "${E2E_IMAGE_REGISTRY_USER}" \
        --docker-password "${E2E_IMAGE_REGISTRY_PASS}" \
        --docker-server "${E2E_IMAGE_REGISTRY_SRV}"
    done
  fi

  set -ex

  # e2e test time limit
  sleep 3600
}

kube_version="$1"

e2e "${kube_version}"
