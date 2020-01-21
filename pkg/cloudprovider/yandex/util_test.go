package yandex_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_GetRegion(t *testing.T) {
	region, err := GetRegion("ru-central1-a")
	assert.Nil(t, err)
	assert.Equal(t, "ru-central1", region)
}

func Test_MapNodeNameToInstanceName(t *testing.T) {
	assert.Equal(t, "e2e-test-node0", MapNodeNameToInstanceName(types.NodeName("e2e-test-node0")))
}

func Test_ParseProviderID(t *testing.T) {
	var folderID string
	var zone string
	var instanceName string
	var err error

	// Valid providerID
	folderID, zone, instanceName, err = ParseProviderID("yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.Nil(t, err)
	assert.Equal(t, folderID, folderID)
	assert.Equal(t, zone, zone)
	assert.Equal(t, instanceName, instanceName)

	// Incorrect providerID
	folderID, zone, instanceName, err = ParseProviderID("fake://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0")
	assert.NotNil(t, err)
}
