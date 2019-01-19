package yandex

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	metadataURL = "http://169.254.169.254"
	userAgent   = "golang/yandex-cloud-controller-manager"
)

// MetadataService knows how to query the Yandex.Cloud instance metadata server.
// See https://cloud.yandex.com/docs/compute/operations/vm-info/get-info#gce-metadata.
type MetadataService struct {
	metadataURL string
	httpClient  *http.Client
}

// NewMetadataService creates an instance of the MetadataService object using default metadata URL.
func NewMetadataService() *MetadataService {
	return NewMetadataServiceWithURL(metadataURL)
}

// NewMetadataServiceWithURL creates an instance of the MetadataService object using specified metadata URL.
func NewMetadataServiceWithURL(metadataURL string) *MetadataService {
	return &MetadataService{
		metadataURL: metadataURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   2 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 2 * time.Second,
			},
		},
	}
}

// GetFolderID returns the current VM's FolderID, such as "b1g4c2a3g6vkffp3qacq"
// Currently instance metadata service does not implement "project/project-id" element, so we using "instance/zone" to extract FolderID information
func (m *MetadataService) GetFolderID() (string, error) {
	result, err := m.Get("instance/zone")
	if err != nil {
		return "", err
	}

	// Metadata contains "instance/zone" in the following form "projects/${folderID}/zones/{zoneName}".
	// So for input "projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a" output will be "b1g4c2a3g6vkffp3qacq".
	parts := strings.Split(result, "/")
	if len(parts) != 4 {
		return "", fmt.Errorf("unexpected input: %s", result)
	}

	return parts[1], nil
}

// GetZone returns the current VM's Zone, such as "ru-central1-a".
func (m *MetadataService) GetZone() (string, error) {
	result, err := m.Get("instance/zone")
	if err != nil {
		return "", err
	}

	// Metadata contains "instance/zone" in the following form "projects/${folderID}/zones/{zoneName}".
	// So for input "projects/b1g4c2a3g6vkffp3qacq/zones/ru-central1-a" output will be "ru-central1-a".
	zone := strings.LastIndex(result, "/")
	if zone == -1 {
		return "", fmt.Errorf("unexpected input: %s", result)
	}

	return result[zone+1:], nil
}

// Get returns a value from the instance metadata service.
// The suffix is appended to "${metadataURL}/computeMetadata/v1/".
func (m *MetadataService) Get(suffix string) (string, error) {
	url := m.metadataURL + "/computeMetadata/v1/" + suffix
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Metadata-Flavor", "Google")
	req.Header.Set("User-Agent", userAgent)
	res, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", fmt.Errorf("unexpected status code while trying to request %s: %d", url, res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
