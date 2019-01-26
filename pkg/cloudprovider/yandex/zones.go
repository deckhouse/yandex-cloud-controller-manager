package yandex

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

// GetZone returns the Zone containing the current zone and locality region for the node we are currently running on.
func (yc *Cloud) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{
		FailureDomain: yc.config.LocalZone,
	}, nil
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID
func (yc *Cloud) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	_, zone, _, err := ParseProviderID(providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{
		FailureDomain: zone,
	}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name.
func (yc *Cloud) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{
		FailureDomain: instance.ZoneId,
	}, nil
}
