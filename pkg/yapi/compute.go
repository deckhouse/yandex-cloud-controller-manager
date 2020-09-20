package yapi

import (
	"context"
	"fmt"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
)

type ComputeService struct {
	cloudCtx *CloudContext

	InstanceSvc compute.InstanceServiceClient
	ZoneSvc     compute.ZoneServiceClient
}

func NewComputeService(iSvc compute.InstanceServiceClient, zSvc compute.ZoneServiceClient,
	cloudCtx *CloudContext) *ComputeService {

	return &ComputeService{
		cloudCtx:    cloudCtx,
		InstanceSvc: iSvc,
		ZoneSvc:     zSvc,
	}
}

func (cs *ComputeService) FindInstanceByName(ctx context.Context, instanceName string) (*compute.Instance, error) {
	result, err := cs.InstanceSvc.List(ctx, &compute.ListInstancesRequest{
		FolderId: cs.cloudCtx.FolderID,
		PageSize: 2,
		Filter:   fmt.Sprintf("name = \"%s\"", instanceName),
	})

	if err != nil {
		return nil, err
	}

	if len(result.Instances) > 1 {
		return nil, fmt.Errorf("more than 1 Instances found by the name %q", instanceName)
	}
	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("no than 1 Instances found by the name %q", instanceName)
	}

	return result.Instances[0], nil
}
