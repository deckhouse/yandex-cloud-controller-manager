package yandex_test

import (
	. "github.com/dlisin/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

var (
	mAPI        *CloudAPIMock
	cloud       *Cloud
	cloudConfig *CloudConfig
)

func beforeCloudTest(config *CloudConfig) {
	mAPI = &CloudAPIMock{}
	cloud = NewCloud(config, mAPI)
	cloudConfig = config
}
