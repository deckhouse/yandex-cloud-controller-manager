package yandex

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func (yc *Cloud) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return yc.extractNodeAddresses(ctx, instance)
}

func (yc *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return []v1.NodeAddress{}, err
	}

	return yc.extractNodeAddresses(ctx, instance)
}

func (yc *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("yandex://%s", instance.Id), nil
}

func (yc *Cloud) InstanceType(_ context.Context, _ types.NodeName) (string, error) {
	return "", nil
}

func (yc *Cloud) InstanceTypeByProviderID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (yc *Cloud) AddSSHKeyToAllInstances(_ context.Context, _ string, _ []byte) error {
	return cloudprovider.NotImplemented
}

func (yc *Cloud) CurrentNodeName(_ context.Context, hostName string) (types.NodeName, error) {
	return types.NodeName(hostName), nil
}

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

func (yc *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return instance.Status == compute.Instance_STOPPED, nil
}

func (yc *Cloud) extractNodeAddresses(ctx context.Context, instance *compute.Instance) ([]v1.NodeAddress, error) {
	if instance.NetworkInterfaces == nil || len(instance.NetworkInterfaces) < 1 {
		return nil, fmt.Errorf("could not find network interfaces for instance: folderID=%s, name=%s", instance.FolderId, instance.Name)
	}

	var nodeAddresses []v1.NodeAddress

	if len(yc.config.InternalNetworkIDsSet) > 0 {
		for _, iface := range instance.NetworkInterfaces {
			networkID, err := mapSubnetIdToNetworkID(ctx, yc.yandexService.VPCSvc.SubnetSvc, iface.SubnetId)
			if err != nil {
				return nil, err
			}

			if _, ok := yc.config.InternalNetworkIDsSet[networkID]; ok {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: iface.PrimaryV4Address.Address})
			}
		}
	} else {
		networkInterface := instance.NetworkInterfaces[0]
		if networkInterface.PrimaryV4Address == nil {
			return nil, fmt.Errorf("could not find primary IPv4 address for instance: folderID=%s, name=%s", instance.FolderId, instance.Name)
		}

		nodeAddresses = []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: networkInterface.PrimaryV4Address.Address}}
		if networkInterface.PrimaryV4Address.OneToOneNat != nil {
			nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: networkInterface.PrimaryV4Address.OneToOneNat.Address})
		}
	}

	if len(yc.config.ExternalNetworkIDsSet) > 0 {
		for _, iface := range instance.NetworkInterfaces {
			networkID, err := mapSubnetIdToNetworkID(ctx, yc.yandexService.VPCSvc.SubnetSvc, iface.SubnetId)
			if err != nil {
				return nil, err
			}

			if _, ok := yc.config.ExternalNetworkIDsSet[networkID]; ok {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: iface.PrimaryV4Address.Address})
			}
		}
	} else {
		for _, iface := range instance.NetworkInterfaces {
			if iface.PrimaryV4Address.OneToOneNat != nil {
				nodeAddresses = append(nodeAddresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: iface.PrimaryV4Address.OneToOneNat.Address})
				break
			}
		}
	}

	return nodeAddresses, nil
}

func (yc *Cloud) getInstanceByProviderID(ctx context.Context, providerID string) (*compute.Instance, error) {
	instanceName, instanceNameIsId, err := ParseProviderID(providerID)
	if err != nil {
		return nil, err
	}

	if instanceNameIsId {
		instance, err := yc.yandexService.ComputeSvc.InstanceSvc.Get(ctx, &compute.GetInstanceRequest{InstanceId: instanceName})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, cloudprovider.InstanceNotFound
			}
			return nil, err
		}
		return instance, nil
	}

	instance, err := yc.yandexService.ComputeSvc.FindInstanceByName(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, cloudprovider.InstanceNotFound
	}

	return instance, nil
}

func (yc *Cloud) getInstanceByNodeName(ctx context.Context, nodeName types.NodeName) (*compute.Instance, error) {
	instanceName := MapNodeNameToInstanceName(nodeName)

	instance, err := yc.yandexService.ComputeSvc.FindInstanceByName(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, cloudprovider.InstanceNotFound
	}

	return instance, nil
}

// TODO: move?
func mapSubnetIdToNetworkID(ctx context.Context, vpcSdk vpc.SubnetServiceClient, subnetID string) (string, error) {
	subnet, err := vpcSdk.Get(ctx, &vpc.GetSubnetRequest{SubnetId: subnetID})
	if err != nil {
		return "", errors.WithStack(err)
	}

	return subnet.NetworkId, nil
}
