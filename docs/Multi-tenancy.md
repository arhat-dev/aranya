# Multi-tenancy

This document, presents serials of sample deployment scripts and commands, shifting management responsibilities of the namespace `foo` to user `bar`, user `bar` will be able to use its edge devices as `Kubernetes` nodes, and no user workload will be scheduled to cluster nodes except the `aranya` deployment.

Shifting management of edge devices to others requires knowledge in `RBAC` world, familiar yourself with `Kubernetes RBAC` [https://kubernetes.io/docs/reference/access-authn-authz/rbac/](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)

- [0. Before you start](#0-before-you-start)
- [1. Setup the role for user `bar`](#1-setup-the-role-for-user-bar)
- [2. Generate CSR for user `bar` (using `cfssl` and `cfssljson`)](#2-generate-csr-for-user-bar-using-cfssl-and-cfssljson)
- [3. Create a `Kubernetes` csr object from the generated `CSR`](#3-create-a-kubernetes-csr-object-from-the-generated-csr)
- [4. Approve the `Kubernetes` csr with `kubectl`](#4-approve-the-kubernetes-csr-with-kubectl)
- [5. Check and get the signed certificate](#5-check-and-get-the-signed-certificate)
- [6. Create `kubeconfig` file for `bar`'s `kubectl`](#6-create-kubeconfig-file-for-bars-kubectl)
- [7. Configure `bar`'s `kubectl` and tryout](#7-configure-bars-kubectl-and-tryout)
- [8. Deploy `aranya` to another namespace](#8-deploy-aranya-to-another-namespace)
- [9. Grant permissions for debuging `arnaya` to user `bar`](#9-grant-permissions-for-debuging-arnaya-to-user-bar)
- [10. Prevent user `bar` from creating pods runing in cluster nodes](#10-prevent-user-bar-from-creating-pods-runing-in-cluster-nodes)
- [Finally, Deploy `EdgeDevice`s and workloads to namespace `foo`](#finally,-deploy-edgedevices-and-workloads-to-namespace-foo)

### 0. Before you start

We need to make sure we have created the right namespace before taking any action.

namespaces required in this example:

- `foo`
  - for user `bar` to manage
- `foo-aranya`
  - to deploy `aranya`

### 1. Setup the role for user `bar`

Create a deployment script (say `bar-rbac.yaml`) with role and role binding for `bar`

```yaml
# filename: bar-rbac.yaml
#
# define allowed actions for `bar` in `foo` namespace
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: foo
  name: bar-role
rules:
- apiGroups: [""]
  resources:
  # allow to create temporary workloads in this namespace
  - pods
  # allow to execute kubectl commands into pods in this namespace
  - pods/exec
  - pods/attach
  - pods/portforward
  - pods/log
  # allow to create in cluster storages in this namespace
  - configmaps
  - secrets
  verbs: ["*"]
- apiGroups: ["apps"]
  resources:
  # allow to create workloads in this namespace
  - deployments
  - daemonsets
  - statefulsets
  verbs: ["*"]
---
# bind role (`bar-role`) with user (`bar`)
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: foo
  name: bar-role-binding
roleRef:
  kind: Role
  name: bar-role
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: User
  name: bar
  apiGroup: rbac.authorization.k8s.io
```

then apply to your `Kubernetes` cluster using kubectl

```bash
kubectl apply -f bar-rbac.yaml
```

### 2. Generate CSR for user `bar` (using `cfssl` and `cfssljson`)

Users in a RBAC applied `Kubernetes` cluster requires a client TLS certificate to access the `kube-apiserver`, and `kube-apiserver` will perform access control procedures to allow or deny user's requests according to the user's role(s) (using certificate field `Common Name` (`CN`) to identify who the user is, and use role binding info to determine its permissions).

So we need to provision a valid `Certificate` for our user `bar` to get authorized by `kube-apiserver`, which must be signed by the same `Certificate Authority` (`CA`) used by `kube-apiserver`, otherwise the `kube-apiserver` will treate this `Certificate` as invalid, and reject connection directly, thus we have to create a `Certificate Signing Request` (`CSR`) to get a signed `Certificate`, which requires a `Private Key` (with its according `Public Key` embedded in) generated using modern crypto algorithms to make it unique and undeniable.

Now, we need to create a `CSR`, with `cfssl`, we only need to write a json spec (say `csr.json`):

- `CN` set to user's name
  - for our user, it's `bar`
- Define which algorithm to use to generate private key
  - here, we use `ecdsa` (`Elliptic Curve Digital Signature Algorithm`) instead of `rsa` (`Rivest–Shamir–Adleman`) for efficiency
- Optionally set the names with custom values to `C`, `ST`, `L`, `O`, `OU`

```json
{
  "CN": "bar",
  "key": {
    "algo": "ecdsa",
    "size": 256
  },
  "names": [
    {
      "C": "country",
      "ST": "state",
      "L": "locality",
      "O": "org",
      "OU": "org-unit"
    }
  ]
}
```

then we are able to generate our `CSR` with `cfssl` and `cfssljson`

```bash
cfssl genkey csr.json | cfssljson -bare bar
```

now we will get a `CSR` file (`bar.csr`) and a `Private Key` file (`bar-key.pem`)

### 3. Create a `Kubernetes` csr object from the generated `CSR`

Since we need to get a (`Certificate`) signed by `CA` as said in the step above, usually we need to get this done by acquiring `CA` file, `Private Key` file of the `CA`, and some configuration to sign the `Certificate` directly, but in `Kubernetes`, we only have to prepare the configuration part:

1. Encode the generated `CSR` with base64 (`cat bar.csr | base64 | tr -d '\n'`)
2. Write a deployment script (say `csr.yaml`) for `Kubernetes` csr, embed the encoded csr into it

    ```yaml
    apiVersion: certificates.k8s.io/v1beta1
    kind: CertificateSigningRequest
    metadata:
      # csr objects are not permanently stored in your `Kubernetes`
      # select a unique name for the time being
      name: user-bar
    spesc:
      groups:
      - system:authenticated
      request: (paste the base64 encoded CSR)
      usages:
      # common usages for most certs
      - digital signature # the reason of the existence of the cert: user identity
      - key encipherment  # securing the key sharing process
      # since the generated cert will only be used for accessing kube-apiserver
      # we do not have to add `server auth` to the usage list
      - client auth
    ```

3. Apply the deployment script with `kubectl`

  ```bash
  kubectl apply -f csr.yaml
  ```

### 4. Approve the `Kubernetes` csr with `kubectl`

We need to get `Kubernetes` csr approved, which is the signal let `Kubernetes` to issue the signed `Certificate` for us.

```bash
kubectl certificate approve user-bar
```

### 5. Check and get the signed certificate

Watch the csr object status with `kubectl get csr user-bar -w`, once there is a `Issued` on your screen, `Kubernetes` has finished the `Certificate` generation, you can acquire it now (base64 encoded).

```bash
kubectl get csr user-bar -o jsonpath='{.status.certificate}'
```

We do not decode the base64 encoded `Certificate` here, we will use it as it is in next step.

### 6. Create `kubeconfig` file for `bar`'s `kubectl`

In the previous step ([#2](#2.-Generate-CSR-for-user-bar-using-cfssl-and-cfssljson)) we have generated `Private Key` along with `CSR`, now we need the base64 encoded `Private Key` (run `cat bar-key.pem | base64 | tr -d '\n'` to get it) and the signed `Certificate` to complete our `kubeconfig`.

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
  name: mycluster
    # If your have applied `Authority CA` sigend certificate for your
    # kube-apiserver, you can set this field to `false`,
    # in most case, you need a `true` for it
    insecure-skip-tls-verify: true
    # Your Kubernetes kube-apiserver's address
    server: https://kube.example.com:6443
contexts:
- context:
  name: mycluser-bar
    cluster: mycluster
    user: bar
current-context: mycluster-bar
preferences: {}
users:
- name: bar
  user:
    client-certificate-data: (paste the base64 encoded certificate here)
    client-key-data: (paste the base64 encoded private key here)
```

### 7. Configure `bar`'s `kubectl` and tryout

```bash
# make an alias to save our time
$ alias k="/path/to/kubectl --kubeconfig=/path/to/bar.kubeconfig -n foo"

# tryout
# $ k get pods
```

### 8. Deploy `aranya` to another namespace

Well, it's discouraged to deploy aranya to one namespace and watch another as stated in [Maintenance.md #Behaviors and Tips](./Maintenance.md#behaviors-and-tips), but to shift management responsibility we should make sure the user in control won't make bad decisions such as deleting the `aranya`'s deployment, so we will deploy `aranya` to another namepspace (say `foo-aranya`) to watch `EdgeDevice`s in namespace `foo`, and the user won't be able to do anything but to see logs of `aranya` (in next step, we will talk about it).

Create a deployment script with all resources required for `aranya`

```yaml
# the service account of aranya, roles and cluster role
# will be bind to it to get aranya work
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: foo-aranya
  name: aranya
---
# aranya's permissions in namespace `foo-aranya`
# if you do not want to expose metrics and do not
# have any EdgeDevice using gRPC as connectivity
# method, just remove it along with role binding
# and set aranya config field `.services.metrics.address`
# to empty string
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: foo-aranya
  name: foo-aranya-role
rules:
- apiGroups: [""]
  resources:
  - services   # to expoese metric service and gRPC services
  - endpoints  # to manage gRPC endpoints
  - pods       # required to find itself
  - configmaps # required to elect leader
  - secrets    # required to store kubelet certificates
  verbs: ["*"]
---
# bind with service account `aranya`
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: foo-aranya
  name: foo-aranya-role-binding
roleRef:
  kind: Role
  name: foo-aranya-role
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: ServiceAccount
  name: aranya
  namespace: foo-aranya
---
# aranya's permissions in namespace `foo` to reconcile EdgeDevices
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: foo
  name: aranya-role
rules:
- apiGroups:
  - ""
  resources:
  - configmaps # required to load custom configmaps for pod deployment
  - secrets  # required to load custom secrets (certs/passwords)
  - pods     # required to watch pods for edge devices
  - pods/status
  - pods/ephemeralcontainers
  - events
  verbs: ["*"]
# control aranya crds
- apiGroups: ["aranya.arhat.dev"]
  resources: ["*"]
  verbs: ["*"]
---
# bind
#   role `aranya-role` in namespace `foo`
# with
#   service account `aranya` in namespace `foo-aranya`
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: foo
  name: aranya-role-binding
roleRef:
  kind: Role
  name: aranya-role
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: ServiceAccount
  name: aranya
  namespace: foo-aranya
---
# aranya's permissions in cluster wide
# to manage node and csr resource objects
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: aranya
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/status
  verbs: ["*"]
# required to generate node certificates automatically
- apiGroups:
  - certificates.k8s.io
  resources:
  - certificatesigningrequests
  # remove the approval ability if you would like to
  # approve on you own (for security reason)
  - certificatesigningrequests/approval
  verbs: ["*"]
- apiGroups:
  - storage.k8s.io
  resources:
  - csinodes
  - csidrivers
  - volumeattachments
  verbs: ["*"]
---
# bind
#   cluster role `aranya`
# with
#   service account `aranya` in namespace `foo-aranya`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: foo-aranya-crb # crb for cluster role binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: aranya
subjects:
- kind: ServiceAccount
  name: aranya
  namespace: foo-aranya
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aranya
  namespace: foo-aranya
data:
  config.yaml: |-
    controller:
      log:
      - level: debug
        file: stderr
        format: console

    virtualnode:
      connectivity:
        # this is the virtual node behavior definition for all EdgeDevices
        # EdgeDevice's spec can override some of these fields
        timers:
          # force close session in server after
          unarySessionTimeout: 10m
          # force sync node status from edge device after
          forceNodeStatusSyncInterval: 10m
          # force sync pod status from edge device after
          forcePodStatusSyncInterval: 10m
      # node status manager
      node:
        timers:
          # should be consistent with your cluster config
          statusSyncInterval: 10s
      # pod status manager and scheduler
      pod:
        timers:
          # interval pod informer to resync cached pods
          resyncInterval: 60s
      # http streams (started by kubectl commands)
      stream:
        timers:
          # close stream when no traffic has been sent/recv for
          # (kubelet default is 4h)
          idleTimeout: 30m
          # cancel stream creation after
          # (kubelet default is 30s)
          creationTimeout: 30s

    services:
      metrics:
        # disable metrics service by default
        address: ""
        # need to select one not used to avoid deployment failure
        port: 10101
---
# this is an example deployment script for test deployment only
# use StatefulSet if possible
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aranya
  namespace: foo-aranya
spec:
  replicas: 1
  selector:
    matchLabels:
      name: aranya
      arhat.dev/role: Controller
  template:
    metadata:
      labels:
        name: aranya
        # required label to get gRPC services working
        arhat.dev/role: Controller
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          # spread pods to different nodes
          - weight: 100
            podAffinityTerm:
              topologyKey: kubernetes.io/hostname
      serviceAccountName: aranya
      # required to work in host network mode to serve kubelet http service
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      volumes:
      - name: config
        configMap:
          name: aranya
          optional: false
      containers:
      - name: aranya
        image: arhatdev/aranya:latest
        imagePullPolicy: Always
        command:
        - /aranya
        env:
        # namespace to watch, `foo` in this example
        - name: WATCH_NAMESPACE
          value: foo
        # the namespace aranya deployed to
        - name: ARANYA_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # the pod name aranya running in
        # used to find aranya itself
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: OPERATOR_NAME
          value: "aranya"
        volumeMounts:
        - name: config
          mountPath: /etc/aranya
        resources:
          limits:
            memory: "100Mi"
```

### 9. Grant permissions for debuging `arnaya` to user `bar`

Create a role and bind it for user `bar` in namespace `foo-aranya` to allow basic read operations

```yaml
# define permissions for `bar` in `foo-aranya` namespace
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: foo-aranya
  name: bar-role
rules:
- apiGroups: [""]
  resources:
  - pods
  - pods/log
  verbs:
  - get
  - watch
---
# bind role (`bar-role`) with user (`bar`)
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  namespace: foo-aranya
  name: bar-role-binding
roleRef:
  kind: Role
  name: bar-role
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: User
  name: bar
  apiGroup: rbac.authorization.k8s.io
```

### 10. Prevent user `bar` from creating pods runing in cluster nodes

User `bar` is expected to manage edge devices, not `Kubernetes` nodes, so no pods created by `bar` should be running in any cluster node, we are currently not able to accomplish this in `Kubernetes` easily, except taint every node with secret taints the user `bar` doesn't know.

So, this is still a TODO for `Kubernetes` to allow namespaced taint to nodes.

### Finally, Deploy `EdgeDevice`s and workloads to namespace `foo`

The cluster admin should deploy `EdgeDevice`s for user `bar`, and user `bar` should be able to connect their edge devices to the cluster with admin's help, then user `bar` can deploy workloads for their edge devices in namespace `foo`.

Now, everyone's happy, user `bar` do not have to concern about the maintenance of `aranya` (the edge device's server) and can access its edge devices from any where at any time via network, and the cluster admin will not bother to care about the access control with the help of `Kubernetes RBAC`.

Hope `aranya` could be the help to save a lot of users without `Kubernetes` users with powerful masters to share their `Kubernetes Masters` with others for edge devices or even computer clusters.
