# Deployment

- [Requirements](#requirements)
- [Deploy arany for Evaluation (using generated default manifests)](#deploy-arany-for-evaluation-using-generated-default-manifests)
- [Deploy aranya for Production (using helm v3)](#deploy-aranya-for-production-using-helm-v3)
- [Deploy your devices](#deploy-your-devices)
- [Deploy workloads](#deploy-workloads)

## Requirements

- `Kubernetes` 1.14+, with `RBAC` and `Node` authorization enabled

__NOTE:__ currently not compatible with `k3s` due to its node reverse-proxy

## Deploy arany for Evaluation (using generated default manifests)

__NOTE:__ This will deploy aranya to the `aranya` namespace only

1. Create `EdgeDevice` CRD definition used by aranya

  ```bash
  kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/master/cicd/deploy/charts/aranya/crds/aranya.arhat.dev_edgedevices.yaml
  ```

1. Deploy aranya to the `aranya` namespace

  ```bash
  kubectl create namespace aranya
  kubectl apply -f https://raw.githubusercontent.com/arhat-dev/aranya/cicd/deploy/kube/aranya.yaml
  ```

## Deploy aranya for Production (using helm v3)

1. Add chart repo `arhat-dev` as noted in [`araht-dev/helm-charts`](https://github.com/arhat-dev/helm-charts)

1. Customize chart values
   - Create a copy of [`values.yaml`](../cicd/deploy/charts/aranya/values.yaml) (say `my-values.yaml`)
   - Edit `my-values.yaml`

1. Install the aranya chart with custom values

  ```bash
  kubectl create namespace ${MY_NAMESPACE}
  helm install ${MY_RELEASE_NAME} arhat-dev/aranya --namespace ${MY_NAMESPACE} -f my-values.yaml
  ```

## Deploy your devices

1. Create `EdgeDevice` resource objects for each one of your devices (see [docs/EdgeDevice](./docs/EdgeDevice.md) for schema reference)
   - aranya will create one Kubernetes Node, Pod (`virtualpod`, in watch namespace) for it
   - `kuebctl logs/exec/cp/attach/port-froward` to the `virtualpod` will work in device host if allowed

1. Deploy [`arhat`][arhat] with proper configuration to your devices
   - [`arhat`][arhat] by default has not pod deployment support, you need to deploy a runtime extension (with proper backend, e.g. `runtime-docker` with `dockerd` running, or `runtime-podman` with `crun` installed) for workload support.
   - [`arhat`][arhat] delegates all network operation to [`abbot`][abbot] by design, if you would like to gain access to cluster network, first you need to deploy aranya along with [`abbot`][abbot] with proper configuration to your Kubernetes cluster, then
     - To access cluster network from your device host, deploy [`abbot`][abbot] to your device host and configure [`arhat`][arhat] with according options.
     - To access cluster network from your workload runtime, deploy [`abbot`][abbot] as a privileged pod to your device runtime using:
         - host network
         - host pid
         - special label: `arhat.dev/role: Abbot`
         - special container name for [`abbot`][abbot] container: [`abbot`][abbot]

__NOTE:__ If you are running [`arhat`][arhat] on linux platform and using container runtime, you only need to deploy abbot as a pod described above for both container and host access to cluster network mesh.

## Deploy workloads

1. Create workloads with tolerations (tolerate taints for node objects of EdgeDevice nodes)

   - Node Taints

      | Taint Key             | Value                         |
      | --------------------- | ----------------------------- |
      | `arhat.dev/namespace` | namespace of the `EdgeDevice` |

1. Assign nodes to `EdgeDevice`s using node selector or node affinity

   - Node Labels

      | Label Name                  | Value                                                                                                   | Customizable |
      | --------------------------- | ------------------------------------------------------------------------------------------------------- | ------------ |
      | `arhat.dev/role`            | `Node`                                                                                                  | N            |
      | `arhat.dev/name`            | name of the `EdgeDevice`                                                                                | N            |
      | `arhat.dev/namespace`       | namespace of the `EdgeDevice`                                                                           | N            |
      | `arhat.dev/arch`            | arch value reported by [arhat][arhat], include details like `armv5` (revision 5), `mipshf` (hard-float) | N            |
      | `{beta.}kubernetes.io/arch` | `GOARCH` derived from `arhat.dev/arch` value                                                            | N            |
      | `kubernetes.io/role`        | namespace of the `EdgeDevice`                                                                           | Y            |

[arhat]: https://github.com/arhat-dev/arhat
[abbot]: https://github.com/arhat-dev/abbot
