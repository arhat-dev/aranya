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

_get_goarch() {
  arch="$1"

  case "${arch}" in
  armv* | arm)
    printf "arm"
    ;;
  x86)
    printf "386"
    ;;
  mips*hf)
    printf "%s" "${arch%hf}"
    ;;
  arm64 | amd64 | ppc64 | ppc64le | riscv64 | s390x | mips*)
    printf "%s" "${arch}"
    ;;
  *)
    echo "unmapped arch ${arch} to goarch" >&2
    exit 1
    ;;
  esac
}

_get_goarm() {
  arch="$1"

  case "${arch}" in
  armv*)
    printf "%d" "${arch#armv}"
    ;;
  *)
    printf ""
    ;;
  esac
}

_get_gomips() {
  arch="$1"

  case "${arch}" in
  mips*hf)
    printf "hardfloat"
    ;;
  mips*)
    printf "softfloat"
    ;;
  *)
    printf ""
    ;;
  esac
}

_get_debian_arch() {
  arch="$1"

  case "${arch}" in
  armv5)
    # armv5 support is rare
    ;;
  armv6)
    printf "armel"
    ;;
  armv7 | arm)
    printf "armhf"
    ;;
  arm64)
    printf "arm64"
    ;;
  x86 | 386)
    printf "i386"
    ;;
  amd64)
    printf "amd64"
    # cross compile to amd64 seems rare
    ;;
  ppc64)
    printf "ppc64"
    ;;
  ppc64le)
    printf "ppc64el"
    ;;
  riscv64 | s390x)
    printf "%s" "${arch}"
    ;;
  mipsle*)
    printf "mipsel"
    ;;
  mips64le*)
    printf "mips64el"
    ;;
  mips*)
    printf "%s" "$(_get_goarch "${arch}")"
    ;;
  *)
    echo "unmapped arch ${arch} to debian arch" >&2
    exit 1
    ;;
  esac
}

_get_debian_triple() {
  arch="$1"

  case "${arch}" in
  armv5)
    # armv5 support is rare
    ;;
  armv6)
    printf "arm-linux-gnueabi"
    ;;
  armv7 | arm)
    printf "arm-linux-gnueabihf"
    ;;
  arm64)
    printf "aarch64-linux-gnu"
    ;;
  x86 | 386)
    printf "i686-linux-gnu"
    ;;
  amd64)
    # cross compile to amd64 seems rare
    ;;
  ppc64)
    printf "powerpc64-linux-gnu"
    ;;
  ppc64le)
    printf "powerpc64le-linux-gnu"
    ;;
  mipsle*)
    printf "mipsel-linux-gnu"
    ;;
  mips64le*)
    printf "mips64el-linux-gnu"
    ;;
  mips*)
    printf "%s-linux-gnu" "$(_get_goarch "${arch}")"
    ;;
  riscv64 | s390x)
    printf "%s-linux-gnu" "${arch}"
    ;;
  *)
    echo "unmapped arch ${arch} to debian triple" >&2
    exit 1
    ;;
  esac
}

_get_alpine_arch() {
  arch="$1"

  case "${arch}" in
  armv5)
    # alpine doesn't have armv5 support
    ;;
  armv6)
    printf "armhf"
    ;;
  armv7 | arm)
    printf "armv7"
    ;;
  arm64)
    printf "aarch64"
    ;;
  x86 | 386)
    printf "x86"
    ;;
  amd64)
    printf "x86_64"
    # cross compile to amd64 seems rare
    ;;
  ppc64)
    printf "ppc64"
    ;;
  ppc64le)
    printf "ppc64le"
    ;;
  mipsle*)
    printf "mipsel"
    ;;
  mips64le*)
    printf "mips64el"
    ;;
  mips*)
    printf "%s" "$(_get_goarch "${arch}")"
    ;;
  riscv64 | s390x)
    printf "%s" "${arch}"
    ;;
  *)
    echo "unmapped arch ${arch} to alpine arch" >&2
    exit 1
    ;;
  esac
}

_get_alpine_triple() {
  arch="$1"

  case "${arch}" in
  armv5)
    # alpine doesn't have armv5 support though
    ;;
  armv6)
    printf "armel-linux-musleabi"
    ;;
  armv7 | arm)
    printf "armv7l-linux-musleabihf"
    ;;
  arm64)
    printf "aarch64-linux-musl"
    ;;
  x86 | 386)
    printf "i686-linux-musl"
    ;;
  amd64)
    # cross compile to amd64 seems rare
    ;;
  ppc64)
    printf "powerpc64-linux-musl"
    ;;
  ppc64le)
    printf "powerpc64le-linux-musl"
    ;;
  mipsle*)
    printf "mipsel-linux-musl"
    ;;
  mips64le*)
    printf "mips64el-linux-musl"
    ;;
  mips*)
    printf "%s-linux-musl" "$(_get_goarch "${arch}")"
    ;;
  riscv64 | s390x)
    printf "%s-linux-musl" "${arch}"
    ;;
  *)
    echo "unmapped arch ${arch} to alpine triple" >&2
    exit 1
    ;;
  esac
}
