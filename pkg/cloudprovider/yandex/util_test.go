package yandex_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_MapNodeNameToInstanceName(t *testing.T) {
	assert.Equal(t, "e2e-test-node0", MapNodeNameToInstanceName(types.NodeName("e2e-test-node0")))
}

func Test_ParseProviderID(t *testing.T) {
	for _, test := range []struct {
		providerID   string
		folderID     string
		zone         string
		instanceName string
		fail         bool
	}{
		{
			providerID:   "yandex://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0",
			folderID:     "b1g4c2a3g6vkffp3qacq",
			zone:         "ru-central1-a",
			instanceName: "e2e-test-node0",
			fail:         false,
		},
		{
			providerID: "fake://b1g4c2a3g6vkffp3qacq/ru-central1-a/e2e-test-node0",
			fail:       true,
		},
	} {
		folderID, zone, instanceName, err := ParseProviderID(test.providerID)
		if test.fail {
			assert.NotNil(t, err)
			assert.Equal(t, "", folderID)
			assert.NotNil(t, "", zone)
			assert.NotNil(t, "", instanceName)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.folderID, folderID)
			assert.Equal(t, test.zone, zone)
			assert.Equal(t, test.instanceName, instanceName)
		}
	}
}
