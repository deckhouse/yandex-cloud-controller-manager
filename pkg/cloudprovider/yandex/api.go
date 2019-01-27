package yandex

import (
	"context"
	"fmt"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

const (
	apiDefaultPageSize = 100
)

// CloudAPI is an abstraction over Yandex.Cloud SDK, to allow mocking/unit testing
type CloudAPI interface {
	// FindInstanceByFolderAndName searches for Instance with the specified folderID and instanceName.
	// If nothing found - no error must be returned.
	FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error)
}

// NewCloudAPI creates new instance of CloudAPI object
func NewCloudAPI(config *CloudConfig) (CloudAPI, error) {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: config.OAuthToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex.Cloud SDK: %s", err)
	}

	return &CloudAPIImpl{
		sdk: sdk,
	}, nil
}

// CloudAPIImpl is an implementation of CloudAPI
type CloudAPIImpl struct {
	sdk *ycsdk.SDK
}

// FindInstanceByFolderAndName searches for Instance with the specified folderID and instanceName.
func (api *CloudAPIImpl) FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error) {
	result, err := api.sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
		FolderId: folderID,
		Filter:   fmt.Sprintf(`%s = "%s"`, "name", instanceName),
		PageSize: apiDefaultPageSize,
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
