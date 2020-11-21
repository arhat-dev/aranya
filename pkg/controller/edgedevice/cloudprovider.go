/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package edgedevice

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"

	"arhat.dev/aranya/pkg/constant"
)

var (
	_ cloudprovider.Interface = (*Controller)(nil)
)

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (c *Controller) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	if _, ok := c.getEdgeDeviceObject(node.Name); !ok {
		return true, fmt.Errorf("no such edge device for node name %q", node.Name)
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (c *Controller) InstanceShutdown(ctx context.Context, node *corev1.Node) (bool, error) {
	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (c *Controller) InstanceMetadata(ctx context.Context, node *corev1.Node) (*cloudprovider.InstanceMetadata, error) {
	addrs, err := c.NodeAddresses(ctx, types.NodeName(node.Name))
	if err != nil {
		return nil, err
	}

	instanceType, err := c.InstanceType(ctx, types.NodeName(node.Name))
	if err != nil {
		return nil, err
	}

	return &cloudprovider.InstanceMetadata{
		ProviderID:    c.getNodeProviderID("", node.Name),
		InstanceType:  instanceType,
		NodeAddresses: addrs,
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c *Controller) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c *Controller) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c, false
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes.
// Also returns true if the interface is supported, false otherwise.
// WARNING: InstancesV2 is an experimental interface and is subject to change in v1.20.
func (c *Controller) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return c, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c *Controller) Instances() (cloudprovider.Instances, bool) {
	return c, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c *Controller) Zones() (cloudprovider.Zones, bool) {
	return c, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *Controller) Clusters() (cloudprovider.Clusters, bool) {
	return c, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *Controller) Routes() (cloudprovider.Routes, bool) {
	return c, false
}

// ProviderName returns the cloud provider ID.
func (c *Controller) ProviderName() string {
	return "aranya"
}

// HasClusterID returns true if a ClusterID is required and set
func (c *Controller) HasClusterID() bool {
	return false
}

var (
	_ cloudprovider.LoadBalancer = &Controller{}
)

// TODO: Break this up into different interfaces (LB, etc) when we have more than one type of service
// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *corev1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (c *Controller) GetLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
) (status *corev1.LoadBalancerStatus, exists bool, err error) {
	return nil, false, cloudprovider.NotImplemented
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *corev1.Service parameter as read-only and not modify it.
func (c *Controller) GetLoadBalancerName(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
) string {
	return ""
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the
// balancer
// Implementations must treat the *corev1.Service and *corev1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (c *Controller) EnsureLoadBalancer(
	ctx context.Context,
	clusterName string,
	service *corev1.Service,
	nodes []*corev1.Node,
) (*corev1.LoadBalancerStatus, error) {
	return nil, cloudprovider.NotImplemented
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *corev1.Service and *corev1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (c *Controller) UpdateLoadBalancer(
	ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	return cloudprovider.NotImplemented
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *corev1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (c *Controller) EnsureLoadBalancerDeleted(
	ctx context.Context, clusterName string, service *corev1.Service) error {
	return cloudprovider.NotImplemented
}

var (
	_ cloudprovider.Instances = &Controller{}
)

// NodeAddresses returns the addresses of the specified instance.
// TODO(roberthbailey): This currently is only used in such a way that it
// returns the address of the calling instance. We should do a rename to
// make this clearer.
func (c *Controller) NodeAddresses(ctx context.Context, name types.NodeName) ([]corev1.NodeAddress, error) {
	if _, ok := c.getEdgeDeviceObject(string(name)); !ok {
		return nil, fmt.Errorf("no such edge device for node name %q", string(name))
	}

	return c.hostNodeAddresses, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (c *Controller) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]corev1.NodeAddress, error) {
	name, err := c.getEdgeDeviceNameFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return c.NodeAddresses(ctx, types.NodeName(name))
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (c *Controller) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	edgeDevice, ok := c.getEdgeDeviceObject(string(nodeName))
	if !ok {
		return "", fmt.Errorf("no such edge device for node name %q", string(nodeName))
	}

	return c.getNodeProviderID(edgeDevice.Namespace, edgeDevice.Name), nil
}

// InstanceType returns the type of the specified instance.
func (c *Controller) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	edgeDevice, ok := c.getEdgeDeviceObject(string(name))
	if !ok {
		return "", fmt.Errorf("no such edge device for node name %q", string(name))
	}

	instanceType := ""
	if len(edgeDevice.Spec.Node.Annotations) > 0 {
		if it, ok := edgeDevice.Spec.Node.Annotations[corev1.LabelInstanceTypeStable]; ok {
			instanceType = it
		} else if it, ok = edgeDevice.Annotations[corev1.LabelInstanceType]; ok {
			instanceType = it
		}
	}

	return instanceType, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (c *Controller) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	name, err := c.getEdgeDeviceNameFromProviderID(providerID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance type from providerID %q: %w", providerID, err)
	}

	return c.InstanceType(ctx, types.NodeName(name))
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (c *Controller) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (c *Controller) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(c.hostNodeName), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (c *Controller) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	name, err := c.getEdgeDeviceNameFromProviderID(providerID)
	if err != nil {
		return true, err
	}

	_, ok := c.getEdgeDeviceObject(name)
	return ok, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (c *Controller) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, err := c.getEdgeDeviceNameFromProviderID(providerID)
	return false, err
}

var (
	_ cloudprovider.Zones = &Controller{}
)

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (c *Controller) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, cloudprovider.NotImplemented
}

// GetZoneByNodeName returns the Zone containing the current zone and
// locality region of the node specified by node name
// This method is particularly used in the context of external cloud
// providers where node initialization must be done outside the kubelets.
func (c *Controller) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	edgeDevice, ok := c.getEdgeDeviceObject(string(nodeName))
	if !ok {
		return cloudprovider.Zone{}, fmt.Errorf("no such edge device for node name %q", string(nodeName))
	}

	failureDomain := ""
	region := ""
	if len(edgeDevice.Spec.Node.Annotations) > 0 {
		if f, ok := edgeDevice.Spec.Node.Annotations[corev1.LabelZoneFailureDomainStable]; ok {
			failureDomain = f
		} else if f, ok = edgeDevice.Annotations[corev1.LabelZoneFailureDomain]; ok {
			failureDomain = f
		}

		if r, ok := edgeDevice.Spec.Node.Annotations[corev1.LabelZoneRegionStable]; ok {
			region = r
		} else if r, ok = edgeDevice.Annotations[corev1.LabelZoneRegion]; ok {
			region = r
		}
	}

	return cloudprovider.Zone{FailureDomain: failureDomain, Region: region}, nil
}

// GetZoneByProviderID returns the Zone containing the current zone and
// locality region of the node specified by providerID
// This method is particularly used in the context of external cloud
// providers where node initialization must be done outside the kubelets.
func (c *Controller) GetZoneByProviderID(
	ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	name, err := c.getEdgeDeviceNameFromProviderID(providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return c.GetZoneByNodeName(ctx, types.NodeName(name))
}

var (
	_ cloudprovider.Clusters = &Controller{}
)

// ListClusters lists the names of the available clusters.
func (c *Controller) ListClusters(ctx context.Context) ([]string, error) {
	return nil, cloudprovider.NotImplemented
}

// Master gets back the address (either DNS name or IP address) of the master node for the cluster.
func (c *Controller) Master(ctx context.Context, clusterName string) (string, error) {
	return "", cloudprovider.NotImplemented
}

var (
	_ cloudprovider.Routes = &Controller{}
)

// ListRoutes lists all managed routes that belong to the specified clusterName
func (c *Controller) ListRoutes(
	ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	return nil, cloudprovider.NotImplemented
}

// CreateRoute creates the described managed route
// route.Name will be ignored, although the cloud-provider may use nameHint
// to create a more user-meaningful name.
func (c *Controller) CreateRoute(
	ctx context.Context, clusterName string, nameHint string, route *cloudprovider.Route) error {
	return cloudprovider.NotImplemented
}

// DeleteRoute deletes the specified managed route
// Route should be as returned by ListRoutes
func (c *Controller) DeleteRoute(
	ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	return cloudprovider.NotImplemented
}

func (c *Controller) getEdgeDeviceNameFromProviderID(providerID string) (string, error) {
	parts := strings.SplitN(providerID, "://", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid providerID %q", providerID)
	}

	if parts[0] != c.ProviderName() {
		// not managed by us, ignore
		return "", fmt.Errorf("unsupported providerID %q", providerID)
	}

	namespacedName := strings.SplitN(parts[1], "/", 2)
	if len(namespacedName) != 2 {
		return "", fmt.Errorf("invalid aranya providerID %q", parts[1])
	}

	if namespacedName[0] != constant.WatchNS() {
		return "", fmt.Errorf("not managed by this aranya provider")
	}

	return namespacedName[1], nil
}

func (c *Controller) getNodeProviderID(namespace, name string) string {
	return fmt.Sprintf("%s://%s/%s", c.ProviderName(), namespace, name)
}
