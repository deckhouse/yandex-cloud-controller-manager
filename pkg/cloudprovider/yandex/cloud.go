package yandex

import (
	"encoding/json"
	"fmt"
	"github.com/yandex-cloud/go-sdk/iamkey"
	"io"
	"os"

	"github.com/pkg/errors"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	providerName = "yandex"

	envServiceAccountJSON = "YANDEX_CLOUD_SERVICE_ACCOUNT_JSON"
	envFolderID           = "YANDEX_CLOUD_FOLDER_ID"
)

// CloudConfig includes all the necessary configuration for creating Cloud object
type CloudConfig struct {
	FolderID    string
	LocalZone   string
	Credentials ycsdk.Credentials
}

// Cloud is an implementation of cloudprovider.Interface for Yandex.Cloud
type Cloud struct {
	api    CloudAPI
	config *CloudConfig
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			config, err := NewCloudConfig()
			if err != nil {
				return nil, err
			}

			api, err := NewCloudAPI(config)
			if err != nil {
				return nil, err
			}

			return NewCloud(config, api), nil
		})
}

// NewCloudConfig creates a new instance of CloudConfig object
func NewCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}
	metadata := NewMetadataService()

	// Retrieve Service Account Json
	saJSON := os.Getenv(envServiceAccountJSON)
	if saJSON == "" {
		return nil, fmt.Errorf("environment variable %q is required", envServiceAccountJSON)
	}
	var iamKey iamkey.Key
	err := json.Unmarshal([]byte(saJSON), &iamKey)
	if err != nil {
		return nil, errors.Wrap(err, "malformed service account json")
	}
	credentials, err := ycsdk.ServiceAccountKey(&iamKey)
	if err != nil {
		return nil, errors.Wrap(err, "invalid auth credentials")
	}

	cloudConfig.Credentials = credentials

	// Retrieve FolderID
	// firstly - try to find it in env. variables
	folderID := os.Getenv(envFolderID)
	if folderID == "" {
		// if env. variable is missing - then fallback to MetadataService
		var err error
		folderID, err = metadata.GetFolderID()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get FolderID from instance metadata")
		}
	}
	cloudConfig.FolderID = folderID

	return cloudConfig, nil
}

// NewCloud creates a new instance of Cloud object
func NewCloud(config *CloudConfig, api CloudAPI) *Cloud {
	return &Cloud{
		api:    api,
		config: config,
	}
}

// Initialize passes a Kubernetes clientBuilder interface to the cloud provider
func (yc *Cloud) Initialize(clientBuilder controller.ControllerClientBuilder) {
}

// LoadBalancer returns a balancer interface if supported.
func (yc *Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface if supported.
func (yc *Cloud) Instances() (cloudprovider.Instances, bool) {
	return yc, true
}

// Zones returns a zones interface if supported.
func (yc *Cloud) Zones() (cloudprovider.Zones, bool) {
	return yc, true
}

// Clusters returns a clusters interface if supported.
func (yc *Cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface if supported
func (yc *Cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (yc *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if the cluster has a clusterID
func (yc *Cloud) HasClusterID() bool {
	return true
}
