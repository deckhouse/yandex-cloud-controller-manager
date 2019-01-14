package yandex

import (
	"io"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	// ProviderName is the value used for the --cloud-provider flag
	ProviderName = "yandex"
)

type cloud struct {
	instances cloudprovider.Instances
	zones     cloudprovider.Zones
}

func init() {
	cloudprovider.RegisterCloudProvider(
		ProviderName,
		func(io.Reader) (cloudprovider.Interface, error) {
			return newCloud()
		})
}

func newCloud() (cloudprovider.Interface, error) {
	return &cloud{
		// ToDo ...
		instances: newInstances(),
		zones:     newZones(),
	}, nil
}

// Initialize passes a Kubernetes clientBuilder interface to the cloud provider
func (c *cloud) Initialize(clientBuilder controller.ControllerClientBuilder) {
}

// LoadBalancer returns a balancer interface if supported.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface if supported.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Zones returns a zones interface if supported.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
}

// Clusters returns a clusters interface if supported.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface if supported
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if the cluster has a clusterID
func (c *cloud) HasClusterID() bool {
	return true
}
