#!/bin/sh

# Copyright 2020 The arhat.dev Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -ex

. scripts/image/common.sh

_ensure_manifest() {
  comp="$1"
  os="$2"
  arch="$3"

  manifest_arch="$(_get_docker_manifest_arch "${arch}")"
  if [ -z "${manifest_arch}" ]; then
    echo "unmapped arch ${arch} to docker manifest arch" >&2
    exit 1
  fi

  args="--os ${os} --arch ${manifest_arch}"

  variant="$(_get_docker_manifest_arch_variant "${arch}")"
  if [ -n "${variant}" ]; then
    args="${args} --variant ${variant}"
  fi

  for repo in ${IMAGE_REPOS}; do
    image_name="$(_get_image_name "${repo}" "${comp}" "${os}" "${arch}")"
    manifest_name="$(_get_image_manifest_name "${repo}" "${comp}")"

    docker manifest create "${manifest_name}" \
      "${image_name}" || true

    docker manifest create "${manifest_name}" \
      --amend "${image_name}"

    # shellcheck disable=SC2086
    docker manifest annotate "${manifest_name}" \
      "${image_name}" ${args}
  done
}

_push_image() {
  comp="$1"
  os="$2"
  arch="$3"

  for repo in ${IMAGE_REPOS}; do
    docker push "$(_get_image_name "${repo}" "${comp}" "${os}" "${arch}")"
  done

  _ensure_manifest "${comp}" "${os}" "${arch}" || true

  for repo in ${IMAGE_REPOS}; do
    docker manifest push "$(_get_image_manifest_name "${repo}" "${comp}")" || true
  done
}

comp=$(printf "%s" "$@" | cut -d\. -f3)
os=$(printf "%s" "$@" | cut -d\. -f4)
arch=$(printf "%s" "$@" | cut -d\. -f5)

_push_image "${comp}" "${os}" "${arch}"
