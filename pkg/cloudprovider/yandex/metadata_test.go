package yandex

import (
	"testing"

	"github.com/dankinder/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_MetadataService_GetFolderID(t *testing.T) {
	mockHandler := &httpmock.MockHandler{}
	mockServer := httpmock.NewServer(mockHandler)
	defer mockServer.Close()

	instanceMetadata := NewMetadataServiceWithURL(mockServer.URL())

	for _, test := range []struct {
		input    string
		folderID string
		fail     bool
	}{
		{"projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a", "b1g4c2a3g6vkffp3qacq", false},
	} {
		mockHandler.On("Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything).Return(httpmock.Response{
			Body: []byte(test.input),
		})

		folderID, err := instanceMetadata.GetFolderID()
		if test.fail {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.folderID, folderID)
		}

		mockHandler.AssertCalled(t, "Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything)
	}
}

func Test_MetadataService_GetZone(t *testing.T) {
	mockHandler := &httpmock.MockHandler{}
	mockServer := httpmock.NewServer(mockHandler)
	defer mockServer.Close()

	instanceMetadata := NewMetadataServiceWithURL(mockServer.URL())

	for _, test := range []struct {
		input string
		zone  string
		fail  bool
	}{
		{"projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a", "ru-central1-a", false},
	} {
		mockHandler.On("Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything).Return(httpmock.Response{
			Body: []byte(test.input),
		})

		zone, err := instanceMetadata.GetZone()
		if test.fail {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.Equal(t, test.zone, zone)
		}

		mockHandler.AssertCalled(t, "Handle", "GET", "/computeMetadata/v1/instance/zone", mock.Anything)
	}
}

func Test_MetadataService_Get(t *testing.T) {
	mockHandler := &httpmock.MockHandler{}
	mockServer := httpmock.NewServer(mockHandler)
	defer mockServer.Close()

	instanceMetadata := NewMetadataServiceWithURL(mockServer.URL())

	for _, test := range []struct {
		key   string
		value string
	}{
		{"instance/id", "fhmjne4n270jqgucjn5i"},
	} {
		mockHandler.On("Handle", "GET", "/computeMetadata/v1/"+test.key, mock.Anything).Return(httpmock.Response{
			Body: []byte(test.value),
		})

		value, err := instanceMetadata.Get(test.key)
		assert.Nil(t, err)
		assert.Equal(t, test.value, value)

		mockHandler.AssertCalled(t, "Handle", "GET", "/computeMetadata/v1/"+test.key, mock.Anything)
	}
}
