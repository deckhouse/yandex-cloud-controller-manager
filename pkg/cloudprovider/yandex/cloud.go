package yandex

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	ycsdk "github.com/yandex-cloud/go-sdk"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	providerName = "yandex"

	envAccessToken = "YANDEX_CLOUD_ACCESS_TOKEN"
	envFolderID    = "YANDEX_CLOUD_FOLDER_ID"

	apiDefaultPageSize = 100
)

// CloudConfig includes all the necessary configuration for creating Cloud object
type CloudConfig struct {
	FolderID   string
	LocalZone  string
	OAuthToken ycsdk.Credentials
}

// Cloud is an implementation of cloudprovider.Interface for Yandex.Cloud
type Cloud struct {
	config *CloudConfig
	sdk    *ycsdk.SDK
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			config, err := NewCloudConfig()
			if err != nil {
				return nil, err
			}

			return NewCloud(config)
		})
}

// NewCloudConfig creates a configuration for yandex.Cloud
func NewCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}
	metadata := NewMetadataService()

	// Retrieve Access Token
	token := os.Getenv(envAccessToken)
	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", envAccessToken)
	}
	cloudConfig.OAuthToken = ycsdk.OAuthToken(token)

	// Retrieve FolderID
	// firstly - try to find it in env. variables
	folderID := os.Getenv(envFolderID)
	if folderID == "" {
		// if env. variable is missing - then fallback to MetadataService
		var err error
		folderID, err = metadata.GetFolderID()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get FolderID from instance metadata")
		}
	}
	cloudConfig.FolderID = folderID

	// Retrieve LocalZone
	localZone, err := metadata.GetZone()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get Zone from instance metadata")
	}
	cloudConfig.LocalZone = localZone

	return cloudConfig, nil
}

// NewCloud creates a new instance of yandex.Cloud
func NewCloud(config *CloudConfig) (cloudprovider.Interface, error) {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: config.OAuthToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex.Cloud SDK: %s", err)
	}

	return &Cloud{
		config: config,
		sdk:    sdk,
	}, nil
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
