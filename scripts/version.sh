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

set -e

GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
GIT_COMMIT="$(git rev-parse HEAD 2>/dev/null || true)"

# VERSION is user specified tag value, will override
GIT_TAG="${VERSION:-$(git describe --tags 2>/dev/null || true)}"
if [ -z "${GIT_TAG}" ]; then
  # no tag detected, fallback to ci system

  GIT_TAG="${CI_COMMIT_TAG}"

  case "${GITHUB_REF}" in
  refs/tags/*)
    GIT_TAG="${GITHUB_REF#refs/tags/}"
    ;;
  esac
fi

if [ -z "${GIT_COMMIT}" ]; then
  # fallback to gitlab-ci or github actions env
  GIT_COMMIT="${CI_COMMIT_SHA:-${GITHUB_SHA}}"
fi

if [ -z "${GIT_BRANCH}" ]; then
  # no branch detected, fallback to ci system

  if [ -n "${GIT_TAG}" ]; then
    GIT_BRANCH="HEAD"
  else
    GIT_BRANCH="${CI_COMMIT_BRANCH}"

    case "${GITHUB_REF}" in
    refs/heads/*)
      GIT_BRANCH="${GITHUB_REF#refs/heads/}"
      ;;
    esac
  fi
fi

if [ -n "$(git status --porcelain 2>/dev/null || true)" ] && [ -z "${VERSION}" ]; then
  # repo not clean, no tag should be used
  GIT_TAG=""
fi
