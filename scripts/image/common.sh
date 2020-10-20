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

. scripts/version.sh

IMAGE_REPOS="$(printf "%s" "${IMAGE_REPOS}" | tr ',' ' ')"
if [ -z "${IMAGE_REPOS}" ]; then
    echo "no image repo provided"
    exit 1
fi

MANIFEST_TAG=""
if [ -n "${GIT_TAG}" ]; then
  # use tag
  MANIFEST_TAG="${GIT_TAG}"
elif [ "${GIT_BRANCH}" = "master" ]; then
  # use default manifest tag for master branch
  MANIFEST_TAG="${DEFAULT_IMAGE_MANIFEST_TAG:-latest}"
elif [ -n "${GIT_BRANCH}" ]; then
  MANIFEST_TAG="$(printf "%s" "${GIT_BRANCH}" | tr '/' '-')"
elif [ -n "${GIT_COMMIT}" ]; then
  MANIFEST_TAG="${GIT_COMMIT}"
fi

_get_docker_manifest_arch() {
  arch="$1"

  case "${arch}" in
  x86)
    printf "386"
    ;;
  armv*)
    # arm32v{5,6,7}
    printf "arm"
    ;;
  arm64)
    printf "arm64"
    ;;
  amd64 | mips64le | ppc64le | s390x)
    printf "%s" "${arch}"
    ;;
  esac
}

_get_docker_manifest_arch_variant() {
  arch="$1"

  case "${arch}" in
  armv*)
    # arm32v{5,6,7}
    printf "%d" "${arch#armv}"
    ;;
  *)
    printf ""
    ;;
  esac
}

_get_tag_prefix_by_os() {
  os="$1"

  case "${os}" in
    windows)
      printf "windows-"
      ;;
    linux)
      printf ""
      ;;
    *)
      echo "unsupported os"
      exit 1
      ;;
  esac
}

_get_image_name() {
  repo="$1"
  comp="$2"
  os="$3"
  arch="$4"

  printf "%s/%s:%s%s-%s" "${repo}" "${comp}" "$(_get_tag_prefix_by_os "${os}")" "${arch}" "${MANIFEST_TAG}"
}

_get_image_manifest_name() {
  repo="$1"
  comp="$2"

  printf "%s/%s:%s" "${repo}" "${comp}" "${MANIFEST_TAG}"
}
