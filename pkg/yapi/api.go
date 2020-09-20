package yapi

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	ycsdk "github.com/yandex-cloud/go-sdk"
	ycsdkoperation "github.com/yandex-cloud/go-sdk/operation"
)

type OperationWaiter func(ctx context.Context, origFunc func() (*operation.Operation, error)) (proto.Message, *ycsdkoperation.Operation, error)

type CloudContext struct {
	RegionID string
	FolderID string

	OperationWaiter OperationWaiter
}

type YandexCloudAPI struct {
	cloudCtx *CloudContext

	VPCSvc     *VPCService
	ComputeSvc *ComputeService
	LbSvc      *LoadBalancerService

	OperationWaiter OperationWaiter
}

func NewYandexCloudAPI(creds ycsdk.Credentials, regionID, folderID string) (*YandexCloudAPI, error) {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{Credentials: creds})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex.Cloud SDK: %s", err)
	}

	opWaiter := func(ctx context.Context, origFunc func() (*operation.Operation, error)) (proto.Message, *ycsdkoperation.Operation, error) {
		op, err := sdk.WrapOperation(origFunc())
		if err != nil {
			return nil, nil, err
		}

		err = op.Wait(ctx)
		if err != nil {
			return nil, op, err
		}

		resp, err := op.Response()
		if err != nil {
			return nil, op, err
		}

		return resp, op, nil
	}

	cloudCtx := &CloudContext{
		RegionID: regionID,
		FolderID: folderID,

		OperationWaiter: opWaiter,
	}

	return &YandexCloudAPI{
		LbSvc:      NewLoadBalancerService(sdk.LoadBalancer().NetworkLoadBalancer(), sdk.LoadBalancer().TargetGroup(), cloudCtx),
		ComputeSvc: NewComputeService(sdk.Compute().Instance(), sdk.Compute().Zone(), cloudCtx),
		VPCSvc:     NewVPCService(sdk.VPC().Network(), sdk.VPC().Subnet(), sdk.VPC().RouteTable(), cloudCtx),
		cloudCtx:   cloudCtx,

		OperationWaiter: opWaiter,
	}, nil
}
