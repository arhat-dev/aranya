module arhat.dev/aranya

go 1.15

require (
	arhat.dev/abbot-proto v0.1.2
	arhat.dev/aranya-proto v0.3.5
	arhat.dev/pkg v0.5.5
	cloud.google.com/go/pubsub v1.3.1
	ext.arhat.dev/runtimeutil v0.3.1
	github.com/Azure/azure-amqp-common-go/v3 v3.0.0
	github.com/Azure/azure-event-hubs-go/v3 v3.3.0
	github.com/Azure/azure-sdk-for-go v45.1.0+incompatible // indirect
	github.com/Azure/go-amqp v0.12.8
	github.com/Azure/go-autorest/autorest/adal v0.9.1 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/container-storage-interface/spec v1.3.0
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/goiiot/libmqtt v0.9.6
	github.com/google/cadvisor v0.38.7
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.6 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/itchyny/gojq v0.12.1
	github.com/klauspost/compress v1.11.7
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/pkg/sftp v1.12.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/streadway/amqp v1.0.0
	github.com/stretchr/testify v1.7.0
	go.uber.org/multierr v1.6.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20210122163508-8081c04a3579 // indirect
	google.golang.org/grpc v1.35.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/apiserver v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/cloud-provider v0.19.7
	k8s.io/cri-api v0.19.7
	k8s.io/csi-translation-lib v0.19.7
	k8s.io/klog/v2 v2.4.0
	k8s.io/kubelet v0.19.7
	k8s.io/kubernetes v1.19.7
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
)

// misc
replace (
	cloud.google.com/go => cloud.google.com/go v0.63.0
	github.com/Microsoft/go-winio => github.com/Microsoft/go-winio v0.4.14
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.8.10
	github.com/OpenPeeDeeP/depguard => github.com/OpenPeeDeeP/depguard v1.0.1
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.1
	github.com/StackExchange/wmi => github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d
	github.com/alecthomas/units => github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d
	github.com/asaskevich/govalidator => github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.31.7
	github.com/aws/aws-sdk-go-v2 => github.com/aws/aws-sdk-go-v2 v0.23.0
	github.com/containerd/typeurl => github.com/containerd/typeurl v1.0.1
	github.com/creack/pty => github.com/creack/pty v1.1.11
	github.com/docker/docker => github.com/docker/engine v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/docker/spdystream => github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c
	github.com/fsnotify/fsnotify => github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf => github.com/golang/protobuf v1.4.2
	github.com/google/gofuzz => github.com/google/gofuzz v1.0.0
	github.com/gorilla/mux => github.com/gorilla/mux v1.8.0
	github.com/itchyny/gojq => github.com/itchyny/gojq v0.12.1
	github.com/jmespath/go-jmespath => github.com/jmespath/go-jmespath v0.3.0
	github.com/pion/dtls/v2 => github.com/pion/dtls/v2 v2.0.4
	github.com/spf13/cobra => github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify => github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink => github.com/vishvananda/netlink v1.1.0
	go.etcd.io/etcd => github.com/etcd-io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	go.uber.org/atomic => github.com/uber-go/atomic v1.7.0
	go.uber.org/zap => github.com/uber-go/zap v1.16.0
	google.golang.org/api => google.golang.org/api v0.36.0
	google.golang.org/appengine => google.golang.org/appengine v1.6.6
	google.golang.org/grpc => github.com/grpc/grpc-go v1.29.1
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	honnef.co/go/tools => github.com/dominikh/go-tools v0.0.1-2020.1.5
	vbom.ml/util => github.com/fvbommel/util v0.0.3
)

// prometheus
replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common => github.com/prometheus/common v0.15.0
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.3.0
)

// go experimental
replace (
	golang.org/x/crypto => github.com/golang/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/exp => github.com/golang/exp v0.0.0-20201229011636-eab1b5eb1a03
	golang.org/x/lint => github.com/golang/lint v0.0.0-20201208152925-83fdc39ff7b5
	golang.org/x/net => github.com/golang/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/oauth2 => github.com/golang/oauth2 v0.0.0-20210113205817-d3ed898aa8a3
	golang.org/x/sync => github.com/golang/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys => github.com/golang/sys v0.0.0-20210124154548-22da62e12c0c
	golang.org/x/text => github.com/golang/text v0.3.5
	golang.org/x/tools => github.com/golang/tools v0.1.0
	golang.org/x/xerrors => github.com/golang/xerrors v0.0.0-20200804184101-5ec99f83aff1
)

// Kubernetes v1.19.7
replace (
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v37.1.0+incompatible
	github.com/container-storage-interface/spec => github.com/container-storage-interface/spec v1.3.0
	github.com/containerd/containerd => github.com/containerd/containerd v1.3.4
	github.com/evanphx/json-patch => github.com/evanphx/json-patch/v5 v5.0.0
	github.com/heketi/heketi => github.com/heketi/heketi v9.0.1-0.20190917153846-c2e2a4ab7ab9+incompatible
	github.com/mindprince/gonvml => github.com/mindprince/gonvml v0.0.0-20190828220739-9ebdce4bb989
	k8s.io/api => github.com/kubernetes/api v0.19.7
	k8s.io/apiextensions-apiserver => github.com/kubernetes/apiextensions-apiserver v0.19.7
	k8s.io/apimachinery => github.com/kubernetes/apimachinery v0.19.7
	k8s.io/apiserver => github.com/kubernetes/apiserver v0.19.7
	k8s.io/cli-runtime => github.com/kubernetes/cli-runtime v0.19.7
	k8s.io/client-go => github.com/kubernetes/client-go v0.19.7
	k8s.io/cloud-provider => github.com/kubernetes/cloud-provider v0.19.7
	k8s.io/cluster-bootstrap => github.com/kubernetes/cluster-bootstrap v0.19.7
	k8s.io/code-generator => github.com/kubernetes/code-generator v0.19.7
	k8s.io/component-base => github.com/kubernetes/component-base v0.19.7
	k8s.io/cri-api => github.com/kubernetes/cri-api v0.19.7
	k8s.io/csi-translation-lib => github.com/kubernetes/csi-translation-lib v0.19.7
	k8s.io/klog => github.com/kubernetes/klog v1.0.0
	k8s.io/klog/v2 => github.com/kubernetes/klog/v2 v2.4.0
	k8s.io/kube-aggregator => github.com/kubernetes/kube-aggregator v0.19.7
	k8s.io/kube-controller-manager => github.com/kubernetes/kube-controller-manager v0.19.7
	k8s.io/kube-proxy => github.com/kubernetes/kube-proxy v0.19.7
	k8s.io/kube-scheduler => github.com/kubernetes/kube-scheduler v0.19.7
	k8s.io/kubectl => github.com/kubernetes/kubectl v0.19.7
	k8s.io/kubelet => github.com/kubernetes/kubelet v0.19.7
	k8s.io/kubernetes => github.com/kubernetes/kubernetes v1.19.7
	k8s.io/legacy-cloud-providers => github.com/kubernetes/legacy-cloud-providers v0.19.7
	k8s.io/metrics => github.com/kubernetes/metrics v0.19.7
	k8s.io/sample-apiserver => github.com/kubernetes/sample-apiserver v0.19.7
	k8s.io/utils => github.com/kubernetes/utils v0.0.0-20210111153108-fddb29f9d009
)

// kubelet
replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.8.0
	github.com/coreos/go-systemd/v22 => github.com/coreos/go-systemd/v22 v22.1.0
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible
	github.com/godbus/dbus/v5 => github.com/godbus/dbus/v5 v5.0.3
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc92
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.3-0.20200520003142-237cc4f519e2
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v1.6.0
)

// azure autorest
replace (
	github.com/Azure/go-amqp => github.com/Azure/go-amqp v0.12.8
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.11.3
	github.com/Azure/go-autorest/autorest/adal => github.com/Azure/go-autorest/autorest/adal v0.9.1
	github.com/Azure/go-autorest/autorest/azure/auth => github.com/Azure/go-autorest/autorest/azure/auth v0.4.0
	github.com/Azure/go-autorest/autorest/date => github.com/Azure/go-autorest/autorest/date v0.2.0
	github.com/Azure/go-autorest/autorest/mocks => github.com/Azure/go-autorest/autorest/mocks v0.3.0
	github.com/Azure/go-autorest/autorest/to => github.com/Azure/go-autorest/autorest/to v0.4.0
	github.com/Azure/go-autorest/autorest/validation => github.com/Azure/go-autorest/autorest/validation v0.2.0
)
