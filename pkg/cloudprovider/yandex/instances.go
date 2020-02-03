package yandex

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider"
)

// NodeAddresses returns the addresses of the node specified by node name.
func (yc *Cloud) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return yc.nodeAddresses(ctx, instance)
}

// NodeAddressesByProviderID returns the addresses of the node specified by providerID
func (yc *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return yc.nodeAddresses(ctx, instance)
}

// InstanceID returns the cloud provider ID of the node with the specified nodeName.
func (yc *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	// instanceID is returned in the following form "${folderID}/${zone}/${instanceName}"
	return instance.FolderId + "/" + instance.ZoneId + "/" + instance.Name, nil
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
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return instance.Status == compute.Instance_STOPPED, nil
}

// nodeAddresses maps the instance information to an array of NodeAddresses
func (yc *Cloud) nodeAddresses(ctx context.Context, instance *compute.Instance) ([]v1.NodeAddress, error) {
	if instance.NetworkInterfaces == nil || len(instance.NetworkInterfaces) < 1 {
		return nil, fmt.Errorf("could not find network interfaces for instance: folderID=%s, name=%s", instance.FolderId, instance.Name)
	}

	var nodeAddresses []v1.NodeAddress

	if len(yc.config.InternalNetworkIDsSet) > 0 {
		for _, iface := range instance.NetworkInterfaces {
			networkID, err := mapSubnetIdToNetworkID(ctx, yc.api.GetSDK(), iface.SubnetId)
			if err != nil {
				return nil, err
			}

			if _, ok := yc.config.InternalNetworkIDsSet[networkID]; ok {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: iface.PrimaryV4Address.Address})
			}
			if _, ok := yc.config.ExternalNetworkIDsSet[networkID]; ok {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: iface.PrimaryV4Address.Address})
			}
		}
	}

	if len(nodeAddresses) > 0 {
		return nodeAddresses, nil
	}

	networkInterface := instance.NetworkInterfaces[0]
	if networkInterface.PrimaryV4Address == nil {
		return nil, fmt.Errorf("could not find primary IPv4 address for instance: folderID=%s, name=%s", instance.FolderId, instance.Name)
	}

	nodeAddresses = []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: networkInterface.PrimaryV4Address.Address}}
	if networkInterface.PrimaryV4Address.OneToOneNat != nil {
		nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: networkInterface.PrimaryV4Address.OneToOneNat.Address})
	}

	return nodeAddresses, nil
}

// getInstanceByProviderID returns Instance with the specified unique providerID
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByProviderID(ctx context.Context, providerID string) (*compute.Instance, error) {
	folderID, _, instanceName, err := ParseProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return yc.getInstanceByFolderAndName(ctx, folderID, instanceName)
}

// getInstanceByNodeName returns Instance with the specified nodeName.
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByNodeName(ctx context.Context, nodeName types.NodeName) (*compute.Instance, error) {
	instanceName := MapNodeNameToInstanceName(nodeName)

	return yc.getInstanceByFolderAndName(ctx, yc.config.FolderID, instanceName)
}

// getInstanceByName returns Instance with the specified folderID and instanceName.
// If the instance is not found - then returning cloudprovider.InstanceNotFound
func (yc *Cloud) getInstanceByFolderAndName(ctx context.Context, folderID, instanceName string) (*compute.Instance, error) {
	instance, err := yc.api.FindInstanceByFolderAndName(ctx, folderID, instanceName)
	if err != nil {
		return nil, err
	}

	if instance == nil {
		return nil, cloudprovider.InstanceNotFound
	}

	return instance, nil
}

func mapSubnetIdToNetworkID(ctx context.Context, sdk *ycsdk.SDK, subnetID string) (string, error) {
	// TODO: cache
	subnet, err := sdk.VPC().Subnet().Get(ctx, &vpc.GetSubnetRequest{SubnetId: subnetID})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return subnet.NetworkId, nil
}
