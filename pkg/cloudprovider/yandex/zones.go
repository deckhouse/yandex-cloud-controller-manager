package yandex

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

type zones struct {
}

func newZones() cloudprovider.Zones {
	return zones{
		// ToDo ...
	}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, errors.New("not implemented") // ToDo ...
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerId
// This method is particularly used in the context of external cloud providers where node initialization must be down
// outside the kubelets.
func (z zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, errors.New("not implemented") // ToDo ...
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name
// This method is particularly used in the context of external cloud providers where node initialization must be down
// outside the kubelets.
func (z zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, errors.New("not implemented") // ToDo ...

}
