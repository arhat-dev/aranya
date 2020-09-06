#!/bin/sh
# shellcheck disable=SC2039

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

. scripts/version.sh
. scripts/build/common.sh

_install_deps() {
  echo "${INSTALL}"
  eval "${INSTALL}"
}

_build() {
  echo "$1"
  eval "$1"
}

template_kubernetes_controller() {
  # TODO: set mandatory tags and predefined tags for specific platforms
  _build "CGO_ENABLED=0 ${GOBUILD} -tags='foo bar ${PREDEFINED_BUILD_TAGS}' ./cmd/template-kubernetes-controller"
}

COMP=$(printf "%s" "$@" | cut -d. -f1)
CMD=$(printf "%s" "$@" | tr '-' '_' | tr '.'  ' ')

# CMD format: {comp} {os} {arch}

GOOS="$(printf "%s" "$@" | cut -d. -f2 || true)"
ARCH="$(printf "%s" "$@" | cut -d. -f3 || true)"

if [ -z "${GOOS}" ] || [ "${GOOS}" = "$(printf "%s" "${COMP}" | tr '-' '_')" ]; then
  # fallback to goos and goarch values
  GOOS="$(go env GOHOSTOS)"
  ARCH="$(go env GOHOSTARCH)"
fi

GOEXE=""
PREDEFINED_BUILD_TAGS=""
case "${GOOS}" in
  darwin)
    PREDEFINED_BUILD_TAGS=""
  ;;
  openbsd)
    PREDEFINED_BUILD_TAGS=""
  ;;
  netbsd)
    PREDEFINED_BUILD_TAGS=""
  ;;
  freebsd)
    PREDEFINED_BUILD_TAGS=""
  ;;
  plan9)
    PREDEFINED_BUILD_TAGS=""
  ;;
  aix)
    PREDEFINED_BUILD_TAGS=""
  ;;
  solaris)
    PREDEFINED_BUILD_TAGS=""
  ;;
  linux)
    case "${ARCH}" in
      mips64* | mipsle*)
        PREDEFINED_BUILD_TAGS=""
      ;;
      riscv64)
        PREDEFINED_BUILD_TAGS=""
      ;;
    esac
  ;;
  windows)
    GOEXE=".exe"
    case "${ARCH}" in
      arm*)
        PREDEFINED_BUILD_TAGS=""
      ;;
    esac
  ;;
esac

CC="gcc"
# STRIP="strip"
CXX="g++"
CFLAGS="-I/usr/include/glib-2.0 -I/usr/include"
LDFLAGS=""

PM_DEB=$(command -v apt-get || printf "")
PM_APK=$(command -v apk || printf "")

if [ -n "${PM_DEB}" ]; then
  TRIPLE="$(_get_debian_triple "${ARCH}")"
  if [ -n "${TRIPLE}" ]; then
    # PKG_CONFIG_PATH="/usr/lib/${TRIPLE}/pkgconfig"
    CFLAGS="-I/usr/include/${TRIPLE} -I/usr/${TRIPLE}/include -I/usr/lib/${TRIPLE}/glib-2.0/include ${CFLAGS}"
    LDFLAGS="-L/lib/${TRIPLE} -L/usr/lib/${TRIPLE}"
  fi

  # TODO: Add required deb packages
  deb_packages=""

  # TODO: inspect why install packages directly will not setup pkgconfig files
  INSTALL="apt-get install -y ${deb_packages}"
  debian_arch="$(_get_debian_arch "${ARCH}")"
  if [ -n "${debian_arch}" ]; then
    packages_with_arch=""
    for pkg in ${deb_packages}; do
      packages_with_arch="${pkg}:${debian_arch} ${packages_with_arch}"
    done
    # TODO: fix install command here
    # INSTALL="${INSTALL} python3-distutils=3.7.3-1 python3-lib2to3=3.7.3-1 python3=3.7.3-1 && apt-get install -y ${packages_with_arch}"
  fi
fi

if [ -n "${PM_APK}" ]; then
  TRIPLE="$(_get_alpine_triple "${ARCH}")"
  if [ -n "${TRIPLE}" ]; then
    # PKG_CONFIG_PATH="/${TRIPLE}/usr/lib/pkgconfig"
    CFLAGS="-I/${TRIPLE}/include -I/${TRIPLE}/usr/include -I/${TRIPLE}/usr/lib/glib-2.0/include ${CFLAGS}"
    LDFLAGS="-L/${TRIPLE}/lib -L/${TRIPLE}/usr/lib"
  fi

  # TODO: Add required apk packages
  apk_packages=""

  INSTALL="apk add ${apk_packages}"
  alpine_arch="$(_get_alpine_arch "${ARCH}")"
  if [ -n "${alpine_arch}" ]; then
    apk_dirs_for_triple=""

    apk_dirs="/var/lib/apk /var/cache/apk /usr/share/apk /etc/apk"
    for d in ${apk_dirs}; do
      apk_dirs_for_triple="/${TRIPLE}${d} ${apk_dirs_for_triple}"
    done

    INSTALL="mkdir -p ${apk_dirs_for_triple} && apk add --root /${TRIPLE} --arch ${alpine_arch} ${apk_packages}"
  fi
fi

if [ -n "${TRIPLE}" ]; then
  CC="${TRIPLE}-gcc"
  CXX="${TRIPLE}-g++"
  # STRIP="${TRIPLE}-strip"
fi

CGO_FLAGS="CC=${CC} CXX=${CXX} CC_FOR_TARGET=${CC} CXX_FOR_TARGET=${CXX} CGO_CFLAGS_ALLOW='-W' CGO_CFLAGS='${CFLAGS}' CGO_LDFLAGS='${LDFLAGS}'"

GO_LDFLAGS="-s -w \
  -X arhat.dev/template-kubernetes-controller/pkg/version.branch=${GIT_BRANCH} \
  -X arhat.dev/template-kubernetes-controller/pkg/version.commit=${GIT_COMMIT} \
  -X arhat.dev/template-kubernetes-controller/pkg/version.tag=${GIT_TAG} \
  -X arhat.dev/template-kubernetes-controller/pkg/version.arch=${ARCH} \
  -X arhat.dev/template-kubernetes-controller/pkg/version.goVersion=$(go version | cut -d\  -f3) \
  -X arhat.dev/template-kubernetes-controller/pkg/version.goCompilerPlatform=$(go version | cut -d\  -f4)"

GOARM=$(_get_goarm "${ARCH}")
if [ -z "${GOARM}" ]; then
  # this can happen if no ARCH specified
  GOARM=$(go env GOARM)
fi

GOMIPS=$(_get_gomips "${ARCH}")
if [ -z "${GOMIPS}" ]; then
  # this can happen if no ARCH specified
  GOMIPS=$(go env GOMIPS)
fi

GOBUILD="GO111MODULE=on GOOS=${GOOS} GOARCH=$(_get_goarch "${ARCH}") \
  GOARM=${GOARM} GOMIPS=${GOMIPS} GOWASM=satconv,signext \
  ${CGO_FLAGS} \
  go build -trimpath -buildmode=${BUILD_MODE:-default} \
  -mod=vendor -ldflags='${GO_LDFLAGS}' -o build/${COMP}.${GOOS}.${ARCH}${GOEXE}"

$CMD
