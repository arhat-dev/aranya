# Development

## Components

- `aranya` (`阿兰若`)
  - Role: The controller to provision virtual nodes and manage EdgeDevices
  - Origin: `aranya` is the remote place where sangha do the spiritual practice (sadhana).
    - `阿兰若` 是 `僧众` 修行的地方, 常位于远离人烟之处

- `arhat` (`阿罗汉`)
  - Role: The reference agent deployed at your edge device, communicate with `aranya` via gRPC or message brokers
  - Origin: `arhat` is the man whose sadhana level is just next to buddha
    - `阿罗汉` 是取得了仅次于佛果位的修行者

- `abbot` (`住持`)
  - Role: A network manager living at your edge device, deployed as a pod or host daemon

- `vihara` (`精舍`)
  - Role: A controller for node related maintenance

## Concepts

- edge device
  - A physical device living inside/outside of your `Kubernetes` cluster network
    - e.g. a RaspberryPi at home, and you're using managed `Kubernetes` service in cloud

- `EdgeDevice`
  - A `Kubernetes` resource type defined with `Custom Resource Definition`(`CRD`)
  - Functionalities:
    - Provide the specification for each one of your edge devices
      - `aranya` will manage a virtual node and connect to message brokers or provision gRPC servers according to the specification for it

- `virtualnode`
  - The node managed by `aranya`, acting like a `kubelet` process
  - Components and its functionalities:
    - connectivity manager
      - communicate with message broker or serve gRPC service to maintain the network connection to edge device
      - signal edge device online/offline to other mangers
    - node manager
      - sync `Node` status periodically in cluster
    - pod manager
      - Schedule pod deployment for edge device
      - handle pod related command execution (`kubectl exec/port-forward/...`)
    - metrics manager
      - handle container/node metrics/stats collection
    - storage manager (optional)

- `virtualpod`
  - The pod created by `aranya`, won't be actually deployed to any node.
  - Functionalities:
    - help `kubectl` to find the edge device, then the device admin could do host management with `kubectl` commands targeting to this pod.

## Conventions

### EdgeDevice related Kubernetes Resources Naming

| Resource Type (with Field)                            | Name Format                                      | Note                                                   |
| ----------------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------ |
| `core/Node.name`                                      | `{EdgeDevice.Name}`                              |                                                        |
| `core/Service.spec.ports[].name`                      | `{EdgeDevice.Name}`                              | only applies to service for grpc based clients         |
| `core/Secret.name`                                    | `kubelet-tls.{host-node.Name}.{EdgeDevice.Name}` | used to cache generated kubelet cert for fast recreate |
| `certificates.k8s.io/CertificateSigningRequests.name` | `kubelet-tls.{host-node.Name}.{EdgeDevice.Name}` |                                                        |

`{host-node.Name}` is the name of the node `aranya` Pod (with leadership) deployed to.

### MQTT Message Topics

Topics for standard MQTT, NOT for any other cloud vendor variants

- `{topicNamespace}/msg`
- `{topicNamespace}/cmd`
- `{topicNamespace}/status`

### CoAP Paths

Topics for CoAP

- `{pathNamespace}/msg`
- `{pathNamespace}/cmd`
- `{pathNamespace}/status`

### AMQP Message Topics (cloud only)

Topics for standard AMQP 0.9

- `{topicNamespace}.msg`
- `{topicNamespace}.cmd`
- `{topicNamespace}.status`
