package yandex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

func Test_Cloud_GetZone(t *testing.T) {
	beforeCloudTest(&CloudConfig{LocalZone: "ru-central1-a"})
	beforeCloudTest(&CloudConfig{LocalZone: "ru-central1-a"})

	zone, err := cloud.GetZone(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, "", zone.Region)
	assert.Equal(t, "ru-central1-a", zone.FailureDomain)
}
