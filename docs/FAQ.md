# Frequently Asked Questions

- [Why not `k3s`](#why-not-k3s)
- [Why not `virtual-kubelet`](#why-not-virtual-kubelet)
- [Why `arhat` and `aranya` (why not kubelet)](#why-arhat-and-aranya-why-not-kubelet)

## Why not `k3s`

[`k3s`][k3s] is really awesome for some kinds of devices, but still requires lots of work to be done to serve all kinds of devices right.

There are some problems we see in k3s, and we don't think problems like that can be resolved in k3s project which would completely change the way k3s works:

- Kubernetes compatibility
  - k3s solved accessibility issue in complex network environments from control plane to worker nodes with a reverse-proxy, which also made it incompatible with upstream Kubernetes, you have to use k3s master to manage k3s agents.
  - k3s depends on container runtime by nature for Kubernetes conformance, but some devices living in critical environments may not be able to run containers, and of course you cannot run k3s there.

- Resource consumption: k3s itself is lightweighted, but when it starts its internal kubelet routine, you need at least 1G memory to serve your applications right.

- Maintenance burden:
  - Management of k3s clusters:
    - When using k3s in your deivces, you are expected to deploy new k3s clusters for each one group of your devices, cluster federation or something like that should be used if you would like them to coordinate with each other
    - Due to the split of clusters we can not simply deploy controllers to cloud hosts and expect them work for your devices.
    - k3s HA is hard to made right just as the case in `Kubernetes`, you probably would not expect HA k3s for remote devices or do not want to deploy multiple HA clusters in network edge, which can add significantly more complexity for maintenance.
  - Management of k3s data plane for splited networks with NAT or Firewall
    - Well, that's not really a big issue since it gains very few attention for now, you can solve this with third party solutions to create a vpn network mesh among you devices before k3s deployment.

In general, we believe k3s is a suitable solution for you, if:

- Your devices have sufficient memory and container runtime support
- You are willing to maintain multiple clusters, user credentials and probably duplicate services (if you already have a standard Kubernetes cluster)

## Why not `virtual-kubelet`

[`virtual-kubelet`][virtual-kubelet] is designed for cloud providers such as `Azure`, `GCP`, `AWS` to run containers at network edge on top of their own container infrastructure (sometimes a huge managed kubernetes cluster).

- A virtual-kubelet is deployed as a pod on behalf of a contaienr runtime, if this model is applied for edge devices, then large scale edge device cluster would claim a lot of pod resource, which requires a lot of node to serve, it's inefficient.

- Currently there is no cluster network support for virtual-kubelet, you have to define you own network infrastructure for your deployment, however, most edge device users aren't able to or don't want to setup such kind of network infrastructure.

## Why `arhat` and `aranya` (why not kubelet)

kubelet is heavily dependent on http and itself is a http server, it's not a good idea for edge devices with poor network to communicate with controller via http and expose network port to the public for debuging purpose (kubectl exec/port-forward).

- `aranya` is the watcher part of kubelet, lots of kubelet's job such as cluster resource fetch and update is done by `aranya`, `aranya` resolves everything in cluster for `arhat` before any command delivered to `arhat`.

- `arhat` is the worker part of kubelet, it's an event driven agent and only tend to command execution.

- Due to the design decisions above, we can gain benefits like:
  - Lower data transmission rate: only need to transfer necessary messages between `aranya` and `arhat` such as pod creation command, container status update, node status update sync etc, keeping the edge device's data usage for management as low as possible.
  - Secuity hardening made easy: No exposed port by default, no cluster credentials stored in remote devices, no ssh needed

[k3s]: https://github.com/k3s-io/k3s
[virtual-kubelet]: https://github.com/virtual-kubelet/virtual-kubelet
