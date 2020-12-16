# aranya `阿兰若`

[![CI](https://github.com/arhat-dev/aranya/workflows/CI/badge.svg)](https://github.com/arhat-dev/aranya/actions?workflow=CI)
[![PkgGoDev](https://pkg.go.dev/badge/arhat.dev/aranya)](https://pkg.go.dev/arhat.dev/aranya)
[![GoReportCard](https://goreportcard.com/badge/arhat.dev/aranya)](https://goreportcard.com/report/arhat.dev/aranya)
[![codecov](https://codecov.io/gh/arhat-dev/aranya/branch/master/graph/badge.svg)](https://codecov.io/gh/arhat-dev/aranya)

The refernce controller implementation of [`aranya-proto`](https://github.com/arhat-dev/aranya-proto), manages all your devices in `Kubernetes`

[![overview](./docs/images/overview.svg)](./docs/images/overview.svg)

## Features

- Integrate all kinds of devices with any operating system into existing `Kubernetes` clusters
  - Reuse your existing cloud infrastructure for devices (Scheduling, Authorization, etc)
  - Group your devices with namespace (soft multi-tenancy), highly scalable (see [docs/Multi-tenancy.md](./docs/Multi-tenancy.md))

- Deploy and manage large scale devices with `kubectl` (Kubernetes API) and `ansible`
  - `kubectl attach/exec/cp/logs/port-forward` to both host and containers
  - `ansible` with `kubectl` connection, see [docs/Ansible.md](./docs/Ansible.md)

- Easy deployment
  - Single custom resource (`EdgeDevice`) to rule all kinds of devices and workloads
  - Edge devices get integrated into `Kubernetes` with lightweight protocols like `MQTT`, `CoAP`

- Flexible cluster network mesh in remote sites as a addon (experimental)
  - Deploy and manage cluster network in device host/runtime with [`abbot`][abbot] container

- Standard `CSI` with remote access (experimental, see [Remote CSI](./docs/Remote-CSI.md))
  - Only mount operation happens in remote devices, everything else is your cloud `Kubernetes` cluster's duty

__NOTE:__ For details of the host management, please refer to [Maintenance #Host Management](./docs/Maintenance.md#host-management)

## Goals

- Make it possible to build absolutely distributed cluster with a single Kubernetes control plane
  - Deploy nodes to anywhere with any kind of network reachability, regardless of IP Address, NAT, Firewall ...
- Extend Kubernetes to be a universal control plane for everything
  - Not only powerful servers, single-board computers, but also embedded devices, micro controllers
- Serve thousands of devices in a Kubernetes cluster at reasonable cost
  - No direct requests to apiserver from nodes
- More secure node deployment
  - No cluster credentials will be stored in nodes
- Offer a more developer friendly way to create custom runtimes using message oriented protocols
  - [`aranya-proto`][aranya-proto] (aranya <-> arhat)
  - [`abbot-proto`][abbot-proto] (aranya <-> arhat (<-> runtime) <-> abbot)
  - [`arhat-proto`][arhat-proto] (arhat <-> runtime)

## Non-Goals

- Simplify Kubernetes
- Create a new Cloud/IoT ecosystem

## FAQ

You may have a lot of questions regarding this project, such as `Why not k3s?`:

see [docs/FAQ.md](./docs/FAQ.md) or file a issue for discussion!

## Project State

EXPERIMENTAL, USE AT YOUR OWN __RISK__

__TL;DR:__ Currently you can treate `aranya` as a management service for `arhat` with prometheus node metrics collecting support

Currently state of functionalities:

- Stable:
  - Port-forward
  - Exec/Attach
  - Node metrics collecting
  - Extended node info reporting and node field hooks
  - MQTT connectivity
  - gRPC connectivity
- Unstable (subject to design change):
  - Role & ClusterRole management
  - GCP Cloud IoT connectivity
  - Azure IoT Hub connectivity
- Experimental (not fully supported):
  - Pod management
  - Network mesh with abbot
  - Remote CSI
  - Credential delivery
  - AMQP 0.9 connectivity
- TODO:
  - NATS Stream connectivity
  - Kafka connectivity
  - AMQP 1.0 connectivity

__NOTE:__ This project lacks tests, all kinds of contribution especially tests are welcome!

### Kubernetes Pod Support

- Pod environment variables source
  - [x] `configMap`
  - [x] `secret`
  - [x] `fieldRef`
  - [x] plain text
- Pod volume
  - source
    - [x] `configMap`
    - [x] `secret`
    - [x] `hostPath`
    - [x] `persistentVolumeClaim` (backed by `CSI` only)
      - [ ] `subPath`
    - [ ] `csi` (inline `CSI`)
  - mount type
    - [x] `VolumeMount`
    - [ ] `VolumeDevice`
- Pod network (requires [`abbot`][abbot])
  - [ ] device host/runtime <-> cluster
  - [ ] device host/runtime <-> device host/runtime

__NOTE:__ You __MUST NOT__ use unsupported features in your pod spec (assigned to `EdgeDevice` nodes), which could result unexpected behavior!

## Deployment

see [docs/Deployment](./docs/Deployment.md) for deployment worklflow of `aranya` and `EdgeDevice`

## Development

see [docs/development](./docs/development/) for topics like `Build`, `Design`, `Releasing`

## LICENSE

```text
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

[abbot]: https://github.com/arhat-dev/abbot
[aranya-proto]: https://github.com/arhat-dev/aranya-proto
[abbot-proto]: https://github.com/arhat-dev/abbot-proto
[arhat-proto]: https://github.com/arhat-dev/arhat-proto
