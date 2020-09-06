# Template Kubernetes Controller

[![CI](https://github.com/arhat-dev/template-kubernetes-controller/workflows/CI/badge.svg)](https://github.com/arhat-dev/template-kubernetes-controller/actions?query=workflow%3ACI) [![GoDoc](https://godoc.org/arhat.dev/template-kubernetes-controller?status.svg)](https://godoc.org/arhat.dev/template-kubernetes-controller) [![GoReportCard](https://goreportcard.com/badge/goiiot/libmqtt)](https://goreportcard.com/report/arhat.dev/template-kubernetes-controller) [![codecov](https://codecov.io/gh/arhat-dev/template-kubernetes-controller/branch/master/graph/badge.svg)](https://codecov.io/gh/arhat-dev/template-kubernetes-controller)

Template for a kubernetes controller

## Make Targets

- binary build: `<comp>.{OS}.{ARCH}`
- image build: `image.build.<comp>.{OS}.{ARCH}`
- image push: `image.push.<comp>.{OS}.{ARCH}`
- unit tests: `test.pkg`, `test.cmd`
- e2e tests: `e2e.v1-16`, `e2e.v1-17`, `e2e.v1-18`
- code generation:
  - generate CRD: `gen.code.<crd group>.<crd version>` (to install required tools: `install.codegen`)
  - generate manifests: `gen.manifests.<comp>`

## LICENSE

```txt
   Copyright 2020 The arhat.dev Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
```
