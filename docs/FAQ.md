# Frequently Asked Questions

## Why not `k3s`

- `k3s` is really awesome for some kind of edge devices, but still requires lots of work to be done to serve all kinds of edge devices right. One of the most significant problems is the splited networks with NAT or Firewall (e.g. the network between your homelab and the cloud provider), and we don't think problems like that should be resolved in `k3s` project which would totally change the way `k3s` works.

- When using `k3s` for your edge deivces, you are expected to deploy new `k3s` clusters to each one group of your edge devices, cluster federation or something like that should be used if you would like them to coordinate with each other, so we can not simply deploy controllers to cloud hosts and expect them work for edge devices. Also, `k3s` HA is hard to made right just as the case in `Kubernetes`, you may not expect HA `k3s` for edge devices or do not want to deploy multiple HA clusters in network edge, that could add additional complexity to maintenance job.

- `k3s` installation requires container runtime, but we may not be able to install container runtime in some embedded devices like home routers.

## Why not `virtual-kubelet`

- `virtual-kubelet` is designed for cloud providers such as `Azure`, `GCP`, `AWS` to run containers at network edge. However, most edge device users aren't able to or don't want to setup such kind of network infrastructure.

- A `virtual-kubelet` is deployed as a pod on behalf of a contaienr runtime, if this model is applied for edge devices, then large scale edge device cluster would claim a lot of pod resource, which requires a lot of node to serve, it's inefficient.

## Why `arhat` and `aranya` (why not `kubelet`)

- `kubelet` is heavily dependent on http and itself is a http server, it's not a good idea for edge devices with poor network to communicate with each other via http and expose network port to the public.

- `aranya` is the watcher part in `kubelet`, lots of `kubelet`'s job such as cluster resource fetch and update is done by `aranya`, `aranya` resolves everything in cluster for `arhat` before any command delivered to `arhat`.

- `arhat` is the worker part in `kubelet`, it's an event driven agent and only tend to command execution.

- Due to the design decisions above, we only need to transfer necessary messages between `aranya` and `arhat` such as pod creation command, container status update, node status update etc. Keeping the edge device's data usage for management as low as possible.
