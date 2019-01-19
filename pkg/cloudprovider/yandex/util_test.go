package yandex

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/types"
)

func Test_mapNodeNameToInstanceName(t *testing.T) {
	assert.Equal(t, "e2e-test-node0", mapNodeNameToInstanceName(types.NodeName("e2e-test-node0")))
}

func Test_parseProviderID(t *testing.T) {
	for _, test := range []struct {
		providerID   string
		zone         string
		instanceName string
		fail         bool
	}{
		{
			providerID:   "yandex:///ru-central1-a/e2e-test-node0",
			zone:         "ru-central1-a",
			instanceName: "e2e-test-node0",
			fail:         false,
		},
		{
			providerID: "fake:///ru-central1-a/e2e-test-node0",
			fail:       true,
		},
	} {
		zone, instanceName, err := parseProviderID(test.providerID)
		if test.fail {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.zone, zone)
			assert.Equal(t, test.instanceName, instanceName)
		}
	}
}
