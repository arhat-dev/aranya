# aranya `阿兰若`

[![Build](https://github.com/arhat-dev/aranya/workflows/CI/badge.svg)](https://github.com/arhat-dev/aranya/actions?workflow=CI) [![GoDoc](https://pkg.go.dev/github.com/arhat-dev/aranya?status.svg)](https://pkg.go.dev/arhat.dev/aranya) [![GoReportCard](https://goreportcard.com/badge/github.com/arhat-dev/aranya)](https://goreportcard.com/report/arhat.dev/aranya) [![codecov](https://codecov.io/gh/arhat-dev/aranya/branch/master/graph/badge.svg)](https://codecov.io/gh/arhat-dev/aranya)

Manages all your devices in your existing `Kubernetes` clusters.

## Features

- Integrate edge devices into existing `Kubernetes` clusters
  - Reuse your existing cloud infrastructure for edge devices (Scheduling, Authorization, etc)
  - Namespace based multi-tenancy, highly scalable (see [docs/Multi-tenancy.md](./docs/Multi-tenancy.md))

- Edge devices get integrated into `Kubernetes` via `MQTT` or `gRPC`
  - `MQTT` support includes standard MQTT 3.1.1, `Azure IoT Hub`, `GCP IoT Core` and `AWS IoT Core`

- Deploy and manage large scale edge devices with `kubectl` and `ansible`
  - `kubectl attach/exec/cp/logs/port-forward` to both host and containers
  - `ansible` with `kubectl` connection, see [docs/Ansible.md](./docs/Ansible.md)

- Easy deployment
  - Even no need to install container runtime if you are going to use `arhat-libpod`
  - `arhat-none` with no container runtime works in most use cases
    - Resource constrained embedded systems like home routers
    - Web browsers with `wasm` support (use `arhat-none.js.wasm` and MQTT via websocket)

- No exposed port on edge devices
  - Avoid attacks caused by misconfigured `ssh`, `scp`
  - The edge device agent `arhat` will not expose any port, but you can epose host ports for containers

- Cluster network as a addon
  - Deploy and manage cluster network in edge devices with `abbot` container
  - Proxy network traffic to cluster via your favorite proxy solution

- `CSI` the remote way (see [Remote CSI](./docs/Remote-CSI.md))
  - Only mount operation happns in edge device, everything else is your cloud `Kubernetes` cluster's duty
  - Dynamic private key generation and deployment for edge devices

- Minimum resource usage
  - Built-in `prometheus` `node_exporter`(unix) / `wmi_exporter`(windows)

__NOTE:__ For details of the host management, please refer to [Maintenance #Host Management](./docs/Maintenance.md#host-management)

## State

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
- Pod network
  - [x] edge device -> cluster
  - [ ] cluster -> edge device
  - [ ] edge device <-> edge device

__NOTE:__ You __MUST NOT__ use unsupported features in your pod spec, which could result unexpected behavior!

## Download

- Multiarch Images (linux/{amd64,x86,arm{v6,v7,64},ppc64le,s390x}):
  - [`docker.io/arhatdev/aranya`](https://hub.docker.com/r/arhatdev/aranya/tags)

## Build

see [docs/Build.md](./docs/Build.md)

## Test

see [docs/Test.md](./docs/Test.md)

## Deployment Requirements

- `Kubernetes` cluster with `RBAC` enabled

## Deployment Workflow

1. Deploy `aranya` to your `Kubernetes` cluster for evaluation with following commands (see [docs/Maintenance.md](./docs/Maintenance.md) for more deployment tips)

   1. Create `CRD`s used by `aranya`

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/deploy/charts/aranya/crds/aranya.arhat.dev_edgedevices.yaml
    ```

   2. Select a `namespace` to deploy `aranya`

    ```bash
    export NS=edge
    ```

   3. Deploy `aranya` to the `edge` namespace

      - Method 1: `kubectl` + YAML manifests (unable to customize anything with this method, so only default settings applied and the deployment namespace will always be `edge`)

      ```bash
      # create the namespace
      $ kubectl create namespace ${NS}

      # deploy
      $ kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/cicd/deploy/kube/aranya.yaml
      ```

      - Method 2: `helm` (v3+)

      ```bash
      # customize chart values first (edit `cicd/deploy/charts/aranya/values.yaml`)
      helm install ${MY_RELEASE_NAME} cicd/deploy/charts/aranya --namespace ${NS}
      ```

2. Create `EdgeDevice` resource objects for each one of your edge devices (see [sample-edge-devices.yaml](./cicd/k8s/sample/sample-edge-devices.yaml) for example)

3. Deploy `arhat` with proper configuration to your edge devices, start and wait to get connected to `aranya`
   - For configuration guide and reference, please refer to [docs/arhat-config.md](./docs/arhat-config.md).
   - Run `arhat` with `/path/to/arhat -c /path/to/arhat-config.yaml` or use some [service scripts](./cicd/scripts)

4. `aranya` will create a `Pod` object inside the `Kubernetes` cluster with the name of the `EdgeDevice` in the same namespace, `kuebctl logs/exec/cp/attach/port-froward` to the `virtualpod` will work in edge device host if allowed. (see design reasons at [Maintenance #Host Management](./docs/Maintenance.md#host-management))

5. `arhat` by default only supports host network mode, if you would like to use container network on your edge devices, you need to deploy `abbot` to your edge devices (please refer to [cicd/k8s/abbot](./cicd/k8s/abbot) for example).

   - __NOTICE:__
     - The `abbot` MUST be deployed in container with
       - Host network namespace
       - Host pid namespace
       - `privileged: true` to execute iptables and create container network
       - Label `arhat.dev/role: Abbot`
     - Reason: We want to simplify the cluster network setup for containers at edge device, and the `abbot` container image contains all dependencies required by the `abbot` (cni plugins, iptables, etc), and `arhat` knows how to provision container network with `abbot`. By this method, we can update both `abbot`(the network manager) and CNI plugins at any time.

6. To access cluster services from your edge device, you have to deploy proxy service in your cluster and proxy client in your edge devices (please refer to [sample-cluster-proxy.yaml](./cicd/k8s/sample/sample-cluster-proxy.yaml) and [sample-device-proxy.yaml](./cicd/k8s/sample/sample-device-proxy.yaml) for example).

7. Create workloads with tolerations (taints for node objects of edge devices) and use node selectors or node affinity to assign to specific edge devices (see [sample-workload.yaml](./cicd/k8s/sample/sample-workload.yaml) for example)

   - Node Taints

      | Taint Key             | Value                         |
      | --------------------- | ----------------------------- |
      | `arhat.dev/namespace` | namespace of the `EdgeDevice` |

   - Node Labels

      | Label Name                  | Value                         | Customizable |
      | --------------------------- | ----------------------------- | ------------ |
      | `arhat.dev/role`            | `Node`                        | N            |
      | `arhat.dev/name`            | name of the `EdgeDevice`      | N            |
      | `arhat.dev/namespace`       | namespace of the `EdgeDevice` | N            |
      | `arhat.dev/arch`            | arch value reported by arhat  | N            |
      | `{beta.}kubernetes.io/arch` | `GOARCH` reported by arhat    | N            |
      | `kubernetes.io/role`        | namespace of the `EdgeDevice` | Y            |

__Note:__ Detailed installation instructions can be found at [docs/Installation.md](./docs/Installation.md)

## Roadmap

see [docs/Roadmap.md](./docs/Roadmap.md)

## FAQ

You may have a lot of questions regarding this project, such as `Why not k3s?`:

see [docs/FAQ.md](./docs/FAQ.md) or file a issue for discussion!

## Troubleshooting

You may encounter problems when using this porject:

see [docs/Troubleshooting.md](./docs/Troubleshooting.md) or seek help from issues

## Special Thanks

- [`Kubernetes`](https://github.com/kubernetes/kubernetes)
  - Really eased my life with my homelab.
- [`virtual-kubelet`](https://github.com/virtual-kubelet/virtual-kubelet)
  - This project was inspired by its idea, which introduced an cloud agent to run containers in network edge.
- `Buddhism`
  - Which is the origin of the name `aranya` and `arhat`.

## Authors

- [Jeffrey Stoke](https://github.com/jeffreystoke)

## License

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
