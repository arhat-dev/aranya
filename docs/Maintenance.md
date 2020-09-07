# Maintenance

## Maintenance Model

`Kubernetes` defines the concept `virtual cluster` via the `namespace` resource type, we will take the advantage of that concept for our maintenance.

1. Both `aranya`'s deployment and `EdgeDevice`s are namespaced (see [Behaviors and Tips](#behaviors-and-tips) for more information).
2. `Node` objects in `Kubernetes` are not namespaced but those managed by `aranya` will contain a taint for `namespace` name.
   - You need to create workloads with tolerations if you do expect the workload to run in edge devices.

## Behaviors and Tips

- `aranya` watches the `namespace` it has been deployed to by default, reconciles `EdgeDevice`s created in the same `namespace`.
  - You can deploy `aranya` to watch some `namespace` other than the one it was deployed to, but is discouraged if you are not working on something stated in [docs/Multi-tenancy.md](./Multi-tenancy.md).

- Only one `aranya` instance in the same `namespace` will work as leader to do the reconcile job, serving `kubelet` servers and connectivity managers.
  - Deploy `aranya` to mutiple `namespace`s if you have quite a lot (tipically more than 500) `EdgeDevice`s to deploy, you should group these `EdgeDevice`s into different `namespace`s with multiple `aranya` instance.

- `aranya` requires host network to work properly.
  - Deploy multiple `aranya` in the same `namespace` to different `Node`s to avoid single point failure, you can achieve this with [`anti-affinity`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity).

- `aranya` will request node certifications for each one of the `EdgeDevice`s you have deployed. The node certification includes the `Node`'s address(es) and hostname(s) (Here the `Node` is the one `aranya` deployed to).
  - Use `StatefulSet` or `nodeAffinity` to avoid unexpected certification regenation when `aranya` is deployed to different nodes.
  - Changes to `Node`'s address(es) or hostname(s) when `aranya` has been serving the `kubelet` servers and connectivity managers (which should be the rare case) for edge devices would result in connectivity failure and remote management failure, you need to restart `aranya` to solve this problem.

## Host Management

`Kubernetes` doesn't allow users to directly control the node via `kubectl`, but with the help of privileged `Pod`, we can do some host maintenance work. (Check out my device plugin [arhat-dev/kube-host-pty](https://github.com/arhat-dev/kube-host-pty) to enable host access via `kubectl` if you are interested)

I would say, it's the best practice for cloud computing to isolate maintenance and deployment with such barrier, but it's not so good in the case of edge devices, because in this way:

If using `ssh` (or `ansible` with `ssh`) for device management (which enables full management):

- Edge devices need to expose ssh service port for remote management
  - Most attack to IoT network happens to ssh services
  - SSH key management in remote devices is another big problem
  - Connectivity through network with NAT and Firewall requires extra effort (such as `frp` service)

If using privileged `Pod` for management:

- We need to maintain a set of management container images
  - To download when needed
    - data usage is a thing
  - To store preflight
    - storage may be a concern

So what if we need to be able to access our edge device at any place and at any time, once the edge device has been online?

`aranya` and `arhat` solves this problem with the concept `virtualpod`

`virtualpod` is a `Pod` object only living in `Kubernetes` cluster and never will be deployed to your edge devices, its phase is always `Running`. Any `kubectl` command to the `virtualpod` such as `logs`, `exec`, `attach` and `port-forward` will be treated as works to do in device host.

No ssh service exposure, no actual `Pod` deployment, maintenance work done right for edge devices ;)

## Upgrading

Thanks to `protobuf`, we are able to upgrade both `aranya` and `arhat` separately and smoothly, without any interruption caused by protocol inconsistency (except when we introduced significant changes to our proto files and it's no longer compatible with older versions.)

To upgrade `aranya`, just use the strategy `Kubernetes` defined for cloud native applications.

__NOTICE:__ `EdgeDevice`s using `grpc` as connectivity method will always experience a temporary connection failure when upgrading `aranya`.

To upgrade `arhat`, we can do upgrade work with the help of `virtualpod` and `kubectl` (say we have a `EdgeDevice` called `my-edge-device` in `NS` namespace):

1. Upload the new `arhat` binary with `kubectl cp` and check digest
   - `kubectl cp /path/to/new/arhat NS/my-edge-device:/path/to/dir`
   - `kubectl -n NS exec my-edge-device -- sha256sum /path/to/dir/arhat`

2. Restart `arhat` daemon

   1. Method 1 (with systemd):
      - `echo "PASSWORD" | kubectl -n NS exec -i my-edge-device -- sudo -S systemctl restart arhat`

   2. Method 2 (with nohup):
      - Find the old `arhat`'s PID
         - `export OLD_ARHAT_PID=$(kubectl -n NS exec my-edge-device -- ps -ax | grep arhat | awk '{ print $1 }')`
         - `echo "${OLD_ARHAT_PID}"`
      - Start the new `arhat` with proper configuration and make sure it will always be runing
         - `kubectl -n NS exec my-edge-device -- nohup /path/to/arhat -c /path/to/arhat-config.yaml &`
         - `kubectl -n NS exec my-edge-device -- ps -ax | grep arhat | wc -l` (should be a `2` there)
         - Watch the new `arhat` process with tools like top
      - Stop the old `arhat` and the new `arhat` will communicate with `Kubernetes` from now on
         - `kubectl -n NS exec my-edge-device -- kill ${OLD_ARHAT_PID}`

__Note:__ You can automate these steps using shell scripts or by extending `ansible` to use `kubectl`, since they are just commands and args.
