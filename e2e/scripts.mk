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

e2e.v1-20:
	sh e2e/suite.sh v1-20

e2e.v1-19:
	sh e2e/suite.sh v1-19

e2e.v1-18:
	sh e2e/suite.sh v1-18

e2e.v1-17:
	sh e2e/suite.sh v1-17

e2e.v1-16:
	sh e2e/suite.sh v1-16

e2e.v1-15:
	sh e2e/suite.sh v1-15

e2e.v1-14:
	sh e2e/suite.sh v1-14

e2e.image.registry:
	docker run -d --name "kind-registry" --restart=always -p "5000:5000" registry:2
