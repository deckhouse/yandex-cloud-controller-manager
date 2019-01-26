package yandex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_Cloud_InstanceID(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
		}, nil).Once()

	instanceID, err := cloud.InstanceID(context.Background(), types.NodeName("e2e-test-node0"))
	assert.Nil(t, err)
	assert.Equal(t, "b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0", instanceID)
}

func Test_Cloud_CurrentNodeName(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{})
	defer afterCloudTest(t)

	nodeName, err := cloud.CurrentNodeName(context.Background(), "e2e-test-node0")
	assert.Nil(t, err)
	assert.Equal(t, types.NodeName("e2e-test-node0"), nodeName)
}
