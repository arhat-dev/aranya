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

install.codegen:
	sh scripts/gen/codegen.sh install_controller_gen
	sh scripts/gen/codegen.sh install_deepcopy_gen

# gen.code.<api group name>.<api group version>
gen.code.aranya.v1alpha1:
	sh scripts/gen/codegen.sh gen $@

gen.code.all: \
	gen.code.aranya.v1alpha1

gen.file.ssh-host-key:
	ssh-keygen -t ed25519 \
		-N "" \
		-C aranya-e2e-test-ssh-host-key \
		-f e2e/testdata/ssh-host-key.pem
