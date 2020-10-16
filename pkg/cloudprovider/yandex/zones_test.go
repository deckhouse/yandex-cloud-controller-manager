package yandex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_Cloud_GetZone(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{LocalZone: "ru-central1-a"})
	defer afterCloudTest(t)

	zone, err := cloud.GetZone(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "ru-central1", zone.Region)
	assert.Equal(t, "ru-central1-a", zone.FailureDomain)
}

func Test_Cloud_GetZoneByProviderID(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	zone, err := cloud.GetZoneByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.Equal(t, "ru-central1", zone.Region)
	assert.Equal(t, "ru-central1-a", zone.FailureDomain)
}

func Test_Cloud_GetZoneByNodeName(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
		}, nil).Once()

	zone, err := cloud.GetZoneByNodeName(context.Background(), types.NodeName("e2e-test-node0"))
	assert.Nil(t, err)
	assert.Equal(t, "ru-central1", zone.Region)
	assert.Equal(t, "ru-central1-a", zone.FailureDomain)
}
