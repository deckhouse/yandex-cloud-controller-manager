package yandex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_Cloud_InstanceID(t *testing.T) {
	beforeCloudTest(&CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})

	for _, test := range []struct {
		nodeName   string
		zone       string
		instanceID string
		fail       bool
	}{
		{"e2e-test-node0", "ru-central1-a", "b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0", false},
		{"e2e-test-node0", "", "", true},
	} {
		if test.fail {
			mAPI.On("FindInstanceByFolderAndName", mock.Anything, cloudConfig.FolderID, test.nodeName).
				Return(nil, errors.New("test error")).Once()
		} else {
			mAPI.On("FindInstanceByFolderAndName", mock.Anything, cloudConfig.FolderID, test.nodeName).
				Return(&compute.Instance{
					FolderId: cloudConfig.FolderID,
					Name:     test.nodeName,
					ZoneId:   test.zone,
				}, nil).Once()
		}

		instanceID, err := cloud.InstanceID(context.Background(), types.NodeName(test.nodeName))
		mAPI.AssertExpectations(t)

		if test.fail {
			assert.NotNil(t, err)
			assert.Equal(t, "", instanceID)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.instanceID, instanceID)
		}
	}
}
