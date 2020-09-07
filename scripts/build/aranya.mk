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
aranya:
	sh scripts/build/build.sh $@

# linux
aranya.linux.amd64:
	sh scripts/build/build.sh $@

aranya.linux.arm64:
	sh scripts/build/build.sh $@

aranya.linux.armv7:
	sh scripts/build/build.sh $@

aranya.linux.armv6:
	sh scripts/build/build.sh $@

aranya.linux.x86:
	sh scripts/build/build.sh $@

aranya.linux.ppc64le:
	sh scripts/build/build.sh $@

aranya.linux.s390x:
	sh scripts/build/build.sh $@

aranya.linux.all: \
	aranya.linux.amd64 \
	aranya.linux.arm64 \
	aranya.linux.armv7 \
	aranya.linux.armv6 \
	aranya.linux.x86 \
	aranya.linux.ppc64le \
	aranya.linux.s390x

# windows
aranya.windows.amd64:
	sh scripts/build/build.sh $@

aranya.windows.armv7:
	sh scripts/build/build.sh $@

aranya.windows.all: \
	aranya.windows.amd64 \
	aranya.windows.armv7
