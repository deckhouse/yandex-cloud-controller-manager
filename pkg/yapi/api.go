package yapi

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	ycsdk "github.com/yandex-cloud/go-sdk"
	ycsdkoperation "github.com/yandex-cloud/go-sdk/operation"
)

type CloudContext struct {
	RegionID string
	FolderID string
}

// YandexCloudAPI is an implementation of CloudAPI
type YandexCloudAPI struct {
	sdk      *ycsdk.SDK
	cloudCtx CloudContext

	LbSvc *YandexLoadBalancerService
}

func (api *YandexCloudAPI) GetSDK() *ycsdk.SDK {
	return api.sdk
}

// NewYandexCloudAPI creates a new instance of YandexCloudAPI
func NewYandexCloudAPI(creds ycsdk.Credentials, regionID, folderID string) (*YandexCloudAPI, error) {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex.Cloud SDK: %s", err)
	}

	cloudCtx := CloudContext{
		RegionID: regionID,
		FolderID: folderID,
	}

	lbSvc := NewYandexLoadBalancerService(sdk, cloudCtx)

	return &YandexCloudAPI{
		sdk:      sdk,
		LbSvc:    lbSvc,
		cloudCtx: cloudCtx,
	}, nil
}

// FindInstanceByFolderAndName searches for Instance with the specified folderID and instanceName.
func (api *YandexCloudAPI) FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error) {
	result, err := api.sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
		FolderId: folderID,
		Filter:   fmt.Sprintf("name = \"%s\"", instanceName),
		PageSize: 2,
	})
	if err != nil {
		return nil, err
	}

	if result.Instances == nil || len(result.Instances) == 0 {
		return nil, nil
	}

	if len(result.Instances) > 1 {
		return nil, fmt.Errorf("multiple instances found: folderID=%s, instanceName=%s", folderID, instanceName)
	}

	return result.Instances[0], nil
}

func waitForResult(ctx context.Context, sdk *ycsdk.SDK, origFunc func() (*operation.Operation, error)) (proto.Message, *ycsdkoperation.Operation, error) {
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
