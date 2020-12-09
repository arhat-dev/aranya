# Development

## Components

- [`aranya`][aranya] (`阿兰若`)
  - Role: The controller to provision virtual nodes and manage EdgeDevices
  - Origin: `aranya` is the remote place where sangha do the spiritual practice (sadhana).
    - `阿兰若` 是 `僧众` 修行的地方, 常位于远离人烟之处

- [`arhat`][arhat] (`阿罗汉`)
  - Role: The reference agent deployed as host daemon in your devices, communicate with `aranya` via gRPC or message brokers
  - Origin: `arhat` is the man whose sadhana level is just next to buddha
    - `阿罗汉` 是取得了仅次于佛果位的修行者

- [`abbot`][abbot] (`住持`)
  - Role: The reference network manager living at remote site, deployed as a pod or host daemon

- [`vihara`][vihara] (`精舍`)
  - Role: The controller to manage node related maintenance

## Concepts

- edge device
  - All kinds of physical devices living inside/outside of your `Kubernetes` cluster network
    - e.g. A OpenWRT router running at home, and you're using managed `Kubernetes` service in cloud

- `EdgeDevice`
  - The CRD defined in [`aranya`][aranya] project
  - Functionalities:
    - Provide the specification for each one of your devices

- `virtualnode`
  - A node process (actually serval goroutines) managed by and running in `aranya`, acting like a real `kubelet` process
  - Components and their functionalities:
    - connectivity manager
      - communicate with message broker or serve gRPC service to work with remote devices
      - signal other mangers with device online/offline
    - node manager
      - sync `Node` status periodically in cluster
    - pod manager
      - Schedule pod deployment for remote devices
      - Handle pod related api requests (`kubectl exec/port-forward/...`)
    - metrics manager
      - Handle collection of container/node metrics/stats
    - storage manager (optional, only enabled when configured)
      - Mount CSI volumes locally and serve mounted volumes for remote devices
    - mesh manager (optional, only enabled when configured)
      - Coordinate all network mesh enabled devices with in cluster [`abbot`][abbot] instances
      - Generate `abbot-proto` bytes to form mesh network among in cluster namespace and remote devices

- `virtualpod`
  - The pod created by `aranya`, won't be actually deployed to any node.
  - Functionalities:
    - Help `kubectl` to find the edge device, then the device admin could do host management with `kubectl` commands targeting to this pod.

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

[aranya]: https://github.com/arhat-dev/aranya
[arhat]: https://github.com/arhat-dev/arhat
[abbot]: https://github.com/arhat-dev/abbot
[vihara]: https://github.com/arhat-dev/vihara
