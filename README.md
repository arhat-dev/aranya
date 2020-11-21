# aranya `阿兰若`

[![Build](https://github.com/arhat-dev/aranya/workflows/CI/badge.svg)](https://github.com/arhat-dev/aranya/actions?workflow=CI)
[![PkgGoDev](https://pkg.go.dev/badge/arhat.dev/aranya)](https://pkg.go.dev/arhat.dev/aranya)
[![GoReportCard](https://goreportcard.com/badge/arhat.dev/aranya)](https://goreportcard.com/report/arhat.dev/aranya)
[![codecov](https://codecov.io/gh/arhat-dev/aranya/branch/master/graph/badge.svg)](https://codecov.io/gh/arhat-dev/aranya)

Manages all your devices in existing `Kubernetes` clusters.

[![overview](./docs/images/overview.svg)](./docs/images/overview.svg)

## Features

- Integrate all kinds of devices with any operating system into existing `Kubernetes` clusters
  - Reuse your existing cloud infrastructure for devices (Scheduling, Authorization, etc)
  - Group your devices with namespace (soft multi-tenancy), highly scalable (see [docs/Multi-tenancy.md](./docs/Multi-tenancy.md))

- Deploy and manage large scale devices with `kubectl` (Kubernetes API) and `ansible`
  - `kubectl attach/exec/cp/logs/port-forward` to both host and containers
  - `ansible` with `kubectl` connection, see [docs/Ansible.md](./docs/Ansible.md)

- Easy deployment
  - Single custom resource (`EdgeDevice`) to rule all kinds of devices
  - Edge devices get integrated into `Kubernetes` with lightweight protocols like `MQTT`, `CoAP`

- Flexible cluster network mesh in remote sites as a addon
  - Deploy and manage cluster network in device host/runtime with [`abbot`][abbot] container

- Stadard `CSI` with remote access (see [Remote CSI](./docs/Remote-CSI.md))
  - Only mount operation happns in remote devices, everything else is your cloud `Kubernetes` cluster's duty
  - Dynamic private key generation and deployment for remote devices

__NOTE:__ For details of the host management, please refer to [Maintenance #Host Management](./docs/Maintenance.md#host-management)

## Project State

EXPERIMENTAL, USE AT YOUR OWN __RISK__

## Kubernetes Pod Support

- Pod environment variables source
  - [x] `configMap`
  - [x] `secret`
  - [x] `fieldRef`
  - [x] plain text
- Pod volume
  - source
    - [x] `configMap`
    - [x] `secret` (bind mount, not tmpfs mount as kubelet will do)
    - [x] `hostPath`
    - [x] `persistentVolumeClaim` (backed by `CSI` only)
      - [ ] `subPath`
    - [ ] `csi` (inline `CSI`)
  - mount type
    - [x] `VolumeMount`
    - [ ] `VolumeDevice`
- Pod network (requires [`abbot`][abbot])
  - [x] device host/runtime <-> cluster
  - [x] device host/runtime <-> device host/runtime

__NOTE:__ You __MUST NOT__ use unsupported features in your pod spec, which could result unexpected behavior!

## Deployment Requirements

- `Kubernetes` cluster with `RBAC` enabled

__NOTE:__ `k3s` currently not compatible with this controller due to its api proxy handling

## Deployment Workflow

1. Deploy `aranya` to your `Kubernetes` cluster for evaluation with following commands (see [docs/Maintenance.md](./docs/Maintenance.md) for more deployment tips)

   - Create `EdgeDevice` CRD definition used by `aranya`

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/deploy/charts/aranya/crds/aranya.arhat.dev_edgedevices.yaml
    ```

   - Select a `namespace` to deploy `aranya`

    ```bash
    export NS=edge
    ```

   - Deploy `aranya` to the `edge` namespace

      - Method 1: `kubectl` + YAML manifests (unable to customize anything with this method, so only default settings applied and the deployment namespace will always be `edge`)

      ```bash
      # create the namespace
      $ kubectl create namespace edge

      # deploy
      $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/cicd/deploy/kube/aranya.yaml
      ```

      - Method 2: `helm` (v3+), add chart repo `arhat-dev` as noted in [`helm-charts`](https://github.com/arhat-dev/helm-charts)

      ```bash
      # customize chart values (edit `values.yaml`)
      helm install ${MY_RELEASE_NAME} arhat-dev/aranya --namespace ${NS}
      ```

2. Create `EdgeDevice` resource objects for each one of your devices (see [docs/EdgeDevice](./docs/EdgeDevice.md))

   - `aranya` will create one kubernetes Node, Pod (`virtualpod`, in watch namespace) for it
   - `kuebctl logs/exec/cp/attach/port-froward` to the `virtualpod` will work in device host if allowed

3. Deploy [`arhat`][arhat] with proper configuration to your devices

4. [`arhat`][arhat] by default has not pod deployment support, you need to deploy a runtime extension (with proper backend, e.g. `runtime-docker` with `dockerd` running, or `runtime-podman` with `crun` installed) for workload support.

5. [`arhat`][arhat] delegates all network operation to [`abbot`][abbot] by design, if you would like to gain access to cluster network, first you need to deploy `aranya` along with [`abbot`][abbot] with proper configuration to your kubernetes cluster, then

   - To access cluster network from your device host, deploy [`abbot`][abbot] to your device host and configure [`arhat`][arhat] with according options.

   - To access cluster network from your workload runtime, deploy [`abbot`][abbot] as a
     - privileged pod using:
       - host network
       - host pid
       - special label: `arhat.dev/role: Abbot`
       - special container name for [`abbot`][abbot] container: [`abbot`][abbot]
     - to your device runtime

6. Create workloads with tolerations (taints for node objects of edge devices) and use node selectors or node affinity to assign pods to specific device

   - Node Taints

      | Taint Key             | Value                         |
      | --------------------- | ----------------------------- |
      | `arhat.dev/namespace` | namespace of the `EdgeDevice` |

   - Node Labels

      | Label Name                  | Value                                                                                           | Customizable |
      | --------------------------- | ----------------------------------------------------------------------------------------------- | ------------ |
      | `arhat.dev/role`            | `Node`                                                                                          | N            |
      | `arhat.dev/name`            | name of the `EdgeDevice`                                                                        | N            |
      | `arhat.dev/namespace`       | namespace of the `EdgeDevice`                                                                   | N            |
      | `arhat.dev/arch`            | arch value reported by [arhat][arhat], contains details like `armv5` (revision 5), `mipshf` (hard-float) | N            |
      | `{beta.}kubernetes.io/arch` | `GOARCH` derived from `arhat.dev/arch`                                                          | N            |
      | `kubernetes.io/role`        | namespace of the `EdgeDevice`                                                                   | Y            |

## Build

see [docs/Build.md](./docs/Build.md)

## Test

see [docs/Test.md](./docs/Test.md)

## Roadmap

see [docs/Roadmap.md](./docs/Roadmap.md)

## FAQ

You may have a lot of questions regarding this project, such as `Why not k3s?`:

see [docs/FAQ.md](./docs/FAQ.md) or file a issue for discussion!

## Special Thanks

- [`Kubernetes`](https://github.com/kubernetes/kubernetes)
  - Really eased my life with my homelab.
- [`virtual-kubelet`](https://github.com/virtual-kubelet/virtual-kubelet)
  - This project was inspired by its idea, which introduced an cloud agent to run containers in network edge.
- `Buddhism`
  - Which is the origin of the name `aranya` and [`arhat`][arhat].

## Authors

- [Jeffrey Stoke](https://github.com/jeffreystoke)

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

[arhat]: https://github.com/arhat-dev/arhat
[abbot]: https://github.com/arhat-dev/abbot
