package yandex_test

import (
	"testing"

	. "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

var (
	mAPI  *CloudAPIMock
	cloud *Cloud
)

func beforeCloudTest(_ *testing.T, config *CloudConfig) {
	mAPI = &CloudAPIMock{}
	cloud = NewCloud(config, mAPI)
}

func afterCloudTest(t *testing.T) {
	mAPI.AssertExpectations(t)
}
