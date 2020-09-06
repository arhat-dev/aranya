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

# native
template-kubernetes-controller:
	sh scripts/build/build.sh $@

# linux
template-kubernetes-controller.linux.amd64:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.arm64:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.armv7:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.armv6:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.x86:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.ppc64le:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.s390x:
	sh scripts/build/build.sh $@

template-kubernetes-controller.linux.all: \
	template-kubernetes-controller.linux.amd64 \
	template-kubernetes-controller.linux.arm64 \
	template-kubernetes-controller.linux.armv7 \
	template-kubernetes-controller.linux.armv6 \
	template-kubernetes-controller.linux.x86 \
	template-kubernetes-controller.linux.ppc64le \
	template-kubernetes-controller.linux.s390x

# windows
template-kubernetes-controller.windows.amd64:
	sh scripts/build/build.sh $@

template-kubernetes-controller.windows.armv7:
	sh scripts/build/build.sh $@

template-kubernetes-controller.windows.all: \
	template-kubernetes-controller.windows.amd64 \
	template-kubernetes-controller.windows.armv7
