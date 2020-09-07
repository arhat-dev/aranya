# Remote CSI

Use cluster `CSI` plugin in edge devices

## Mechanism

- `virtualnode` is responsible for mounting `PersistentVolume` in cluster for your edge devices
- edge device uses lightweight network FUSE solutions (e.g. `sshfs`) to mount `PersistentVolume` from `virtualnode`

## Pros

- Simple and easy to deploy
  - No `CSI` plugin deployed to any of your edge device while you can use all the `CSI` plugins available in your `Kubernetes` cluster
    - Good for resource constrained devices
- Secure
  - Always using private key authorization and keys are deployed to edge devices dynamically

## Cons

- Storage throughput is limited by (great to little):
  - downlink speed of edge device
  - uplink speed of the node in cluster deployed with `aranya` Pod (with leadership)
  - optimization of `aranya`'s remote CSI implementation
