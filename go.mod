module arhat.dev/template-kubernetes-controller

go 1.15

replace (
	k8s.io/api => github.com/kubernetes/api v0.18.8
	k8s.io/apiextensions-apiserver => github.com/kubernetes/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery => github.com/kubernetes/apimachinery v0.18.8
	k8s.io/apiserver => github.com/kubernetes/apiserver v0.18.8
	k8s.io/cli-runtime => github.com/kubernetes/cli-runtime v0.18.8
	k8s.io/client-go => github.com/kubernetes/client-go v0.18.8
	k8s.io/cloud-provider => github.com/kubernetes/cloud-provider v0.18.8
	k8s.io/cluster-bootstrap => github.com/kubernetes/cluster-bootstrap v0.18.8
	k8s.io/code-generator => github.com/kubernetes/code-generator v0.18.8
	k8s.io/component-base => github.com/kubernetes/component-base v0.18.8
	k8s.io/cri-api => github.com/kubernetes/cri-api v0.18.8
	k8s.io/csi-translation-lib => github.com/kubernetes/csi-translation-lib v0.18.8
	k8s.io/klog => github.com/kubernetes/klog v1.0.0
	k8s.io/klog/v2 => github.com/kubernetes/klog/v2 v2.3.0
	k8s.io/kube-aggregator => github.com/kubernetes/kube-aggregator v0.18.8
	k8s.io/kube-controller-manager => github.com/kubernetes/kube-controller-manager v0.18.8
	k8s.io/kube-proxy => github.com/kubernetes/kube-proxy v0.18.8
	k8s.io/kube-scheduler => github.com/kubernetes/kube-scheduler v0.18.8
	k8s.io/kubectl => github.com/kubernetes/kubectl v0.18.8
	k8s.io/kubelet => github.com/kubernetes/kubelet v0.18.8
	k8s.io/kubernetes => github.com/kubernetes/kubernetes v1.18.8
	k8s.io/legacy-cloud-providers => github.com/kubernetes/legacy-cloud-providers v0.18.8
	k8s.io/metrics => github.com/kubernetes/metrics v0.18.8
	k8s.io/sample-apiserver => github.com/kubernetes/sample-apiserver v0.18.8
	k8s.io/utils => github.com/kubernetes/utils v0.0.0-20200821003339-5e75c0163111
)

require (
	arhat.dev/pkg v0.0.0-20200814094720-eac4d5d5680b
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect
)
