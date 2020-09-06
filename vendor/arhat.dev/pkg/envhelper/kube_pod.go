// +build !nocloud,!nokube

package envhelper

import (
	"os"
)

const (
	EnvKeyPodName      = "POD_NAME"
	EnvKeyPodNamespace = "POD_NAMESPACE"
)

var (
	podName string
	podNS   string
)

func init() {
	var ok bool
	podName = os.Getenv(EnvKeyPodName)

	podNS, ok = os.LookupEnv(EnvKeyPodNamespace)
	if !ok {
		podNS = "default"
	}
}

func ThisPodName() string {
	return podName
}

func ThisPodNS() string {
	return podNS
}
