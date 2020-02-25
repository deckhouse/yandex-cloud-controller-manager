package yapi_test

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
)

// CloudAPIMock is a mock implementation of CloudAPI
type CloudAPIMock struct {
	mock.Mock
}

// FindInstanceByFolderAndName is a mock implementation of CloudAPI.FindInstanceByFolderAndName
func (m *CloudAPIMock) FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error) {
	args := m.Called(ctx, folderID, instanceName)

	var result *compute.Instance
	var err error

	switch args.Get(0).(type) {
	case *compute.Instance:
		result = args.Get(0).(*compute.Instance)
	}

	switch args.Get(1).(type) {
	case error:
		err = args.Get(1).(error)
	}

	return result, err
}
