package yandex_test

import (
	"testing"

	"github.com/dankinder/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	. "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
)

var (
	mHandler        *httpmock.MockHandler
	mServer         *httpmock.Server
	metadataService *MetadataService
)

func beforeMetadataServiceTest(_ *testing.T) {
	mHandler = &httpmock.MockHandler{}
	mServer = httpmock.NewServer(mHandler)
	metadataService = NewMetadataServiceWithURL(mServer.URL())
}

func afterMetadataServiceTest(t *testing.T) {
	mHandler.AssertExpectations(t)
	mServer.Close()
}

func Test_MetadataService_GetFolderID(t *testing.T) {
	beforeMetadataServiceTest(t)
	defer afterMetadataServiceTest(t)

	mHandler.On("Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything).Return(httpmock.Response{
		Body: []byte("projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a"),
	})

	folderID, err := metadataService.GetFolderID()
	assert.Nil(t, err)
	assert.Equal(t, "b1g4c2a3g6vkffp3qacq", folderID)
}

func Test_MetadataService_GetZone(t *testing.T) {
	beforeMetadataServiceTest(t)
	defer afterMetadataServiceTest(t)

	mHandler.On("Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything).Return(httpmock.Response{
		Body: []byte("projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a"),
	})

	zone, err := metadataService.GetZone()
	assert.Nil(t, err)
	assert.Equal(t, "ru-central1-a", zone)
}

func Test_MetadataService_Get(t *testing.T) {
	beforeMetadataServiceTest(t)
	defer afterMetadataServiceTest(t)

	mHandler.On("Handle", "GET", "/computeMetadata/v1/instance/name", mock.Anything).Return(httpmock.Response{
		Body: []byte("e2e-test-node0"),
	})

	value, err := metadataService.Get("instance/name")
	assert.Nil(t, err)
	assert.Equal(t, "e2e-test-node0", value)
}
