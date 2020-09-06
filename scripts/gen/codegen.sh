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

GOPATH=$(go env GOPATH)
export GOPATH

CONTROLLER_GEN="${GOPATH}/bin/kube-controller-gen"

_install_go_bin() {
  package="$1"
  cmd_dir="$2"
  bin="$3"

  # download
  temp_dir="$(mktemp -d)"
  cd "${temp_dir}"
  GO111MODULE=on go get -d -u "${package}"
  cd -
  rmdir "${temp_dir}"

  # build
  cd "${GOPATH}/pkg/mod/${package}"
  GO111MODULE=on go build -o "${bin}" "${cmd_dir}"
  cd -
}

_do_sync_gopath() {
  mkdir -p "${GOPATH}/src/arhat.dev"
  rsync -avh "$(pwd)" "${GOPATH}/src/arhat.dev/"
}

install_deepcopy_gen() {
  _install_go_bin "k8s.io/code-generator@v0.18.8" "./cmd/client-gen" "${GOPATH}/bin/client-gen"
  _install_go_bin "k8s.io/code-generator@v0.18.8" "./cmd/lister-gen" "${GOPATH}/bin/lister-gen"
  _install_go_bin "k8s.io/code-generator@v0.18.8" "./cmd/informer-gen" "${GOPATH}/bin/informer-gen"
}

install_controller_gen() {
  _install_go_bin "sigs.k8s.io/controller-tools@v0.3.0" "./cmd/controller-gen" "${CONTROLLER_GEN}"
}

_do_gen_clients() {
  group_name="$1"
  group_version="$2"

  mkdir -p build

  bash "${GOPATH}/pkg/mod/k8s.io/code-generator@v0.18.8/generate-groups.sh" client,lister,informer \
    "arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated" \
    arhat.dev/template-kubernetes-controller/pkg/apis "${group_name}:${group_version}" \
    --go-header-file "$(pwd)/scripts/gen/boilerplate.go.txt" \
    --plural-exceptions "Maintenance:Maintenance" \
    -v 2 2>&1 | tee build/gen_clients.log | grep -E -e '(Assembling)|(violation)'

  rm -rf "./pkg/apis/${group_name}/generated"
  mkdir -p "./pkg/apis/${group_name}/generated"

  if [ -d "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/clientset" ]; then
    mv "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/clientset" "./pkg/apis/${group_name}/generated"
  fi

  if [ -d "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/informers" ]; then
    mv "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/informers" "./pkg/apis/${group_name}/generated"
  fi

  if [ -d "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/listers" ]; then
    mv "${GOPATH}/src/arhat.dev/template-kubernetes-controller/pkg/apis/${group_name}/generated/listers" "./pkg/apis/${group_name}/generated"
  fi
}

_do_gen_deepcopy() {
  group_name="$1"

  "${CONTROLLER_GEN}" object:headerFile=./scripts/gen/boilerplate.go.txt paths="./pkg/apis/${group_name}/..."
}

_do_gen_crd_manifests() {
  group_name="$1"

	"${CONTROLLER_GEN}" crd:preserveUnknownFields=true output:dir=./cicd/deploy/charts/template-kubernetes-controller/crds/ paths="./pkg/apis/${group_name}/..."
}

gen() {
  cmd=$(printf "%s" "$@" | tr '.'  ' ')

  group_name="$(printf "%s" "${cmd}" | cut -d\  -f3)"
  group_version="$(printf "%s" "${cmd}" | cut -d\  -f4)"

  _do_gen_deepcopy "${group_name}"
  _do_gen_crd_manifests "${group_name}"

  _do_sync_gopath

  _do_gen_clients "${group_name}" "${group_version}"
}

# shellcheck disable=SC2068
$@
