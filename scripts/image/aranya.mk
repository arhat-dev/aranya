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

# build
image.build.aranya.linux.x86:
	sh scripts/image/build.sh $@

image.build.aranya.linux.amd64:
	sh scripts/image/build.sh $@

image.build.aranya.linux.armv6:
	sh scripts/image/build.sh $@

image.build.aranya.linux.armv7:
	sh scripts/image/build.sh $@

image.build.aranya.linux.arm64:
	sh scripts/image/build.sh $@

image.build.aranya.linux.ppc64le:
	sh scripts/image/build.sh $@

image.build.aranya.linux.s390x:
	sh scripts/image/build.sh $@

image.build.aranya.linux.all: \
	image.build.aranya.linux.amd64 \
	image.build.aranya.linux.arm64 \
	image.build.aranya.linux.armv7 \
	image.build.aranya.linux.armv6 \
	image.build.aranya.linux.x86 \
	image.build.aranya.linux.s390x \
	image.build.aranya.linux.ppc64le

image.build.aranya.windows.amd64:
	sh scripts/image/build.sh $@

image.build.aranya.windows.armv7:
	sh scripts/image/build.sh $@

image.build.aranya.windows.all: \
	image.build.aranya.windows.amd64 \
	image.build.aranya.windows.armv7

# push
image.push.aranya.linux.x86:
	sh scripts/image/push.sh $@

image.push.aranya.linux.amd64:
	sh scripts/image/push.sh $@

image.push.aranya.linux.armv6:
	sh scripts/image/push.sh $@

image.push.aranya.linux.armv7:
	sh scripts/image/push.sh $@

image.push.aranya.linux.arm64:
	sh scripts/image/push.sh $@

image.push.aranya.linux.ppc64le:
	sh scripts/image/push.sh $@

image.push.aranya.linux.s390x:
	sh scripts/image/push.sh $@

image.push.aranya.linux.all: \
	image.push.aranya.linux.amd64 \
	image.push.aranya.linux.arm64 \
	image.push.aranya.linux.armv7 \
	image.push.aranya.linux.armv6 \
	image.push.aranya.linux.x86 \
	image.push.aranya.linux.s390x \
	image.push.aranya.linux.ppc64le

image.push.aranya.windows.amd64:
	sh scripts/image/push.sh $@

image.push.aranya.windows.armv7:
	sh scripts/image/push.sh $@

image.push.aranya.windows.all: \
	image.push.aranya.windows.amd64 \
	image.push.aranya.windows.armv7
