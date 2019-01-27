package yandex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_Cloud_NodeAddresses(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					Index: "0",
					PrimaryV4Address: &compute.PrimaryAddress{
						Address: "172.20.0.10",
						OneToOneNat: &compute.OneToOneNat{
							Address: "84.201.125.225",
						},
					},
				},
			},
		}, nil).Once()

	nodeAddresses, err := cloud.NodeAddresses(context.Background(), types.NodeName("e2e-test-node0"))
	assert.Nil(t, err)
	assert.ElementsMatch(t, []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: "172.20.0.10"},
		{Type: v1.NodeExternalIP, Address: "84.201.125.225"},
	}, nodeAddresses)
}

func Test_Cloud_NodeAddressesByProviderID(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	var nodeAddresses []v1.NodeAddress
	var err error

	// Instance has only InternalIP
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					Index: "0",
					PrimaryV4Address: &compute.PrimaryAddress{
						Address: "172.20.0.10",
					},
				},
			},
		}, nil).Once()

	nodeAddresses, err = cloud.NodeAddressesByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.ElementsMatch(t, []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: "172.20.0.10"},
	}, nodeAddresses)

	// Instance has both InternalIP & ExternalIP
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					Index: "0",
					PrimaryV4Address: &compute.PrimaryAddress{
						Address: "172.20.0.10",
						OneToOneNat: &compute.OneToOneNat{
							Address: "84.201.125.225",
						},
					},
				},
			},
		}, nil).Once()

	nodeAddresses, err = cloud.NodeAddressesByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.ElementsMatch(t, []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: "172.20.0.10"},
		{Type: v1.NodeExternalIP, Address: "84.201.125.225"},
	}, nodeAddresses)
}

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

func Test_Cloud_InstanceExistsByProviderID(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	var exists bool
	var err error

	// Instance exists
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
		}, nil).Once()

	exists, err = cloud.InstanceExistsByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.True(t, exists)

	// Instance does not exists
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(nil, nil).Once()

	exists, err = cloud.InstanceExistsByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.False(t, exists)
}

func Test_Cloud_InstanceShutdownByProviderID(t *testing.T) {
	beforeCloudTest(t, &CloudConfig{FolderID: "b1g4c2a3g6vkffp3qacq"})
	defer afterCloudTest(t)

	var shutdown bool
	var err error

	// Instance status -> STOPPED
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
			Status:   compute.Instance_STOPPED,
		}, nil).Once()

	shutdown, err = cloud.InstanceShutdownByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.True(t, shutdown)

	// Instance status -> RUNNING
	mAPI.On("FindInstanceByFolderAndName", mock.Anything, "b1g4c2a3g6vkffp3qacq", "e2e-test-node0").
		Return(&compute.Instance{
			FolderId: "b1g4c2a3g6vkffp3qacq",
			Name:     "e2e-test-node0",
			ZoneId:   "ru-central1-a",
			Status:   compute.Instance_RUNNING,
		}, nil).Once()

	shutdown, err = cloud.InstanceShutdownByProviderID(context.Background(), "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.False(t, shutdown)
}
