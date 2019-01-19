package yandex

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

// NodeAddresses returns the addresses of the node specified by node name.
func (yc *Cloud) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return extractNodeAddresses(instance)
}

// NodeAddressesByProviderID returns the addresses of the node specified by providerID
func (yc *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return extractNodeAddresses(instance)
}

// InstanceID returns the cloud provider ID of the node with the specified nodeName.
func (yc *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	// instanceID is returned in the following form "${zone}/${instanceID}"
	return instance.ZoneId + "/" + instance.Id, nil
}

// InstanceType returns the type of the node with the specified nodeName.
// Currently "" is always returned, since Yandex.Cloud API does not provide any information about instance type.
func (yc *Cloud) InstanceType(ctx context.Context, nodeName types.NodeName) (string, error) {
	return "", nil
}

// InstanceTypeByProviderID returns the type of the node with the specified unique providerD.
// Currently "" is always returned, since Yandex.Cloud API does not provide any information about instance type.
func (yc *Cloud) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	return "", nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances.
func (yc *Cloud) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
func (yc *Cloud) CurrentNodeName(ctx context.Context, hostName string) (types.NodeName, error) {
	return types.NodeName(hostName), nil
}

// InstanceExistsByProviderID returns true if the instance with the given providerID still exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
func (yc *Cloud) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is in safe state to detach volumes
func (yc *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, cloudprovider.NotImplemented
}

// getInstanceByProviderID returns Instance with the specified unique providerID
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByProviderID(ctx context.Context, providerID string) (*compute.Instance, error) {
	_, instanceName, err := parseProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return yc.getInstanceByName(ctx, instanceName)
}

// getInstanceByName returns Instance with the specified nodeName.
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByNodeName(ctx context.Context, nodeName types.NodeName) (*compute.Instance, error) {
	instanceName := mapNodeNameToInstanceName(nodeName)

	return yc.getInstanceByName(ctx, instanceName)
}

// getInstanceByName returns Instance with the specified instanceName.
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByName(ctx context.Context, instanceName string) (*compute.Instance, error) {
	result, err := yc.sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
		FolderId: yc.config.FolderID,
		Filter:   fmt.Sprintf(`%s = "%s"`, "name", instanceName),
		PageSize: apiDefaultPageSize,
	})

	if err != nil {
		return nil, errors.Wrapf(err, "cannot list ")
	}

	if result.Instances != nil || len(result.Instances) > 0 {
		// If more then one instance is found - returning first one
		return result.Instances[0], nil
	}

	return nil, cloudprovider.InstanceNotFound
}
