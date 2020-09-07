package pod

import (
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/pkg/apis/core/pods"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/pkg/kubelet/envvars"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"
)

// ResolveEnv resolves all required pod environment variables
// nolint:gocyclo
func (m *Manager) resolveEnv(
	pod *corev1.Pod,
	ctr *corev1.Container,
) (map[string]string, error) {
	var (
		result     = make(map[string]string)
		configMaps = make(map[string]*corev1.ConfigMap)
		secrets    = make(map[string]*corev1.Secret)
		tmpEnv     = make(map[string]string)
		err        error
	)

	enableServiceLinks := pod.Spec.EnableServiceLinks == nil || *pod.Spec.EnableServiceLinks
	if enableServiceLinks {
		for _, e := range envvars.FromServices(m.options.ListServices()) {
			result[e.Name] = e.Value
		}
	}

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "KUBERNETES_SERVICE") || strings.HasPrefix(env, "KUBERNETES_PORT") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	// Env will override EnvFrom variables.
	// Process EnvFrom first then allow Env to replace existing values.
	for _, envFrom := range ctr.EnvFrom {
		switch {
		case envFrom.ConfigMapRef != nil:
			cm := envFrom.ConfigMapRef
			name := cm.Name
			configMap, ok := configMaps[name]
			if !ok {
				optional := cm.Optional != nil && *cm.Optional
				configMap = m.options.GetConfigMap(name)
				if configMap == nil {
					if optional {
						continue
					}

					return result, fmt.Errorf("failed to get configmap %q", name)
				}
				configMaps[name] = configMap
			}

			var invalidKeys []string
			for k, v := range configMap.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := validation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				tmpEnv[k] = v
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
			}
		case envFrom.SecretRef != nil:
			s := envFrom.SecretRef
			name := s.Name
			secret, ok := secrets[name]
			if !ok {
				optional := s.Optional != nil && *s.Optional
				secret = m.options.GetSecret(name)
				if secret == nil {
					if optional {
						continue
					}
					return result, fmt.Errorf("failed to get secret %q", name)
				}
				secrets[name] = secret
			}

			var invalidKeys []string
			for k, v := range secret.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := validation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				tmpEnv[k] = string(v)
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				// kl.recorder.Eventf(pod, v1.EventTypeWarning, "InvalidEnvironmentVariableNames",
				// 		"Keys [%s] from the EnvFrom secret %s/%s were skipped since
				// 		they are considered invalid environment variable names.",
				// 		strings.Join(invalidKeys, ", "), pod.Namespace, name)
			}
		}
	}

	// Determine the final values of variables:
	//
	// 1.  Determine the final value of each variable:
	//     a.  If the variable's Value is set, expand the `$(var)` references to other
	//         variables in the .Value field; the sources of variables are the declared
	//         variables of the container and the service environment variables
	//     b.  If a source is defined for an environment variable, resolve the source
	// 2.  Create the container's environment in the order variables are declared
	// 3.  Add remaining service environment vars
	var (
		mappingFunc = expansion.MappingFuncFor(tmpEnv)
	)
	for _, envVar := range ctr.Env {
		runtimeVal := envVar.Value
		if runtimeVal != "" {
			// Step 1a: expand variable references
			runtimeVal = expansion.Expand(runtimeVal, mappingFunc)
		} else if envVar.ValueFrom != nil {
			// Step 1b: resolve alternate env var sources
			switch {
			case envVar.ValueFrom.FieldRef != nil:
				runtimeVal, err = podFieldSelectorRuntimeValue(envVar.ValueFrom.FieldRef, pod)
				if err != nil {
					return result, err
				}
			case envVar.ValueFrom.ResourceFieldRef != nil:
				// defaultedPod, defaultedContainer, err := kl.defaultPodLimitsForDownwardAPI(pod, container)
				// if err != nil {
				// 	return result, err
				// }
				runtimeVal, err = containerResourceRuntimeValue(envVar.ValueFrom.ResourceFieldRef, pod, ctr)
				if err != nil {
					return result, err
				}
			case envVar.ValueFrom.ConfigMapKeyRef != nil:
				cm := envVar.ValueFrom.ConfigMapKeyRef
				name := cm.Name
				key := cm.Key
				optional := cm.Optional != nil && *cm.Optional
				configMap, ok := configMaps[name]
				if !ok {
					configMap = m.options.GetConfigMap(name)
					if configMap == nil {
						if optional {
							continue
						}

						return result, fmt.Errorf("failed to get configmap %q", name)
					}
					configMaps[name] = configMap
				}
				runtimeVal, ok = configMap.Data[key]
				if !ok {
					if optional {
						continue
					}
					return result, fmt.Errorf("failed to find key %v in ConfigMap %v/%v", key, pod.Namespace, name)
				}
			case envVar.ValueFrom.SecretKeyRef != nil:
				s := envVar.ValueFrom.SecretKeyRef
				name := s.Name
				key := s.Key
				optional := s.Optional != nil && *s.Optional
				secret, ok := secrets[name]
				if !ok {
					secret = m.options.GetSecret(name)
					if secret == nil {
						if optional {
							continue
						}
						return result, fmt.Errorf("failed to get secret %q", name)
					}
					secrets[name] = secret
				}
				runtimeValBytes, ok := secret.Data[key]
				if !ok {
					if optional {
						continue
					}
					return result, fmt.Errorf("failed to find key %v in Secret %v/%v", key, pod.Namespace, name)
				}
				runtimeVal = string(runtimeValBytes)
			}
		}

		tmpEnv[envVar.Name] = runtimeVal
	}

	// Append the env vars
	for k, v := range tmpEnv {
		result[k] = v
	}

	return result, nil
}

// containerResourceRuntimeValue returns the value of the provided container resource
func containerResourceRuntimeValue(
	fs *corev1.ResourceFieldSelector,
	pod *corev1.Pod,
	container *corev1.Container,
) (string, error) {
	containerName := fs.ContainerName
	if len(containerName) == 0 {
		return resource.ExtractContainerResourceValue(fs, container)
	}
	return resource.ExtractResourceValueByContainerName(fs, pod, containerName)
}

// podFieldSelectorRuntimeValue returns the runtime value of the given
// selector for a pod.
func podFieldSelectorRuntimeValue(fs *corev1.ObjectFieldSelector, pod *corev1.Pod) (string, error) {
	internalFieldPath, _, err := pods.ConvertDownwardAPIFieldLabel(fs.APIVersion, fs.FieldPath, "")
	if err != nil {
		return "", err
	}
	switch internalFieldPath {
	case "spec.nodeName":
		return pod.Spec.NodeName, nil
	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName, nil
	case "status.hostIP":
		return pod.Status.HostIP, nil
	case "status.podIP":
		return pod.Status.PodIP, nil
	}
	return fieldpath.ExtractFieldPathAsString(pod, internalFieldPath)
}
