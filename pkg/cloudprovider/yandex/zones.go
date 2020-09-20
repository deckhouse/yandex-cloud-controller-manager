package yandex

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func (yc *Cloud) GetZone(_ context.Context) (cloudprovider.Zone, error) {
	return yc.getZone(yc.config.LocalZone)
}

func (yc *Cloud) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	instance, err := yc.getInstanceByProviderID(ctx, providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return yc.getZone(instance.ZoneId)
}

func (yc *Cloud) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	instance, err := yc.getInstanceByNodeName(ctx, nodeName)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return yc.getZone(instance.ZoneId)
}

func (yc *Cloud) getZone(zone string) (cloudprovider.Zone, error) {
	region, err := GetRegion(zone)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{
		Region:        region,
		FailureDomain: zone,
	}, nil
}
