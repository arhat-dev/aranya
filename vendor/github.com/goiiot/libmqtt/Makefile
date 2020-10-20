# Copyright Go-IIoT (https://github.com/goiiot)
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

.PHONY: test lib client clean fuzz_test

TEST_FLAGS=-v -count=1 -race -mod=readonly -coverprofile=coverage.txt -covermode=atomic

test_reconnect:
	go test ${TEST_FLAGS} -tags offline -run=TestClient_Reconnect

test:
	go test ${TEST_FLAGS} -run=.

test_auth:
	go test ${TEST_FLAGS} -run=TestAuth

test_conn:
	go test ${TEST_FLAGS} -run=TestConn
	go test ${TEST_FLAGS} -run=TestDisConn

test_ping:
	go test ${TEST_FLAGS} -run=TestPing

test_pub:
	go test ${TEST_FLAGS} -run=TestPub

test_sub:
	go test ${TEST_FLAGS} -run=TestSub
	go test ${TEST_FLAGS} -run=TestUnSub

.PHONY: all_lib c_lib java_lib py_lib \
		clean_c_lib clean_java_lib clean_py_lib

all_lib: c_lib java_lib

clean_all_lib: clean_c_lib clean_java_lib

c_lib:
	$(MAKE) -C c lib

clean_c_lib:
	$(MAKE) -C c clean

java_lib:
	$(MAKE) -C java build

clean_java_lib:
	$(MAKE) -C java clean

client:
	$(MAKE) -C cmd build

clean: clean_all_lib fuzz_clean
	rm -rf coverage.txt

fuzz_test:
	go-fuzz-build github.com/goiiot/libmqtt
	go-fuzz -bin=./libmqtt-fuzz.zip -workdir=fuzz-test

fuzz_clean:
	rm -rf fuzz-test libmqtt-fuzz.zip

fmt:
	golangci-lint run --fix ./...
