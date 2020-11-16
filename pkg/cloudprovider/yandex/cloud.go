package yandex

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	v1 "k8s.io/client-go/listers/core/v1"

	"github.com/flant/yandex-cloud-controller-manager/pkg/yapi"

	mapset "github.com/deckarep/golang-set"

	"k8s.io/client-go/informers"

	"k8s.io/client-go/tools/cache"

	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/pkg/errors"
	ycsdk "github.com/yandex-cloud/go-sdk"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	providerName = "yandex"

	envClusterName        = "YANDEX_CLUSTER_NAME"
	envRouteTableID       = "YANDEX_CLOUD_ROUTE_TABLE_ID"
	envServiceAccountJSON = "YANDEX_CLOUD_SERVICE_ACCOUNT_JSON"
	envFolderID           = "YANDEX_CLOUD_FOLDER_ID"
	envLbListenerSubnetID = "YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID"
	envLbTgNetworkID      = "YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID"
	envInternalNetworkIDs = "YANDEX_CLOUD_INTERNAL_NETWORK_IDS"
	envExternalNetworkIDs = "YANDEX_CLOUD_EXTERNAL_NETWORK_IDS"
)

// CloudConfig includes all the necessary configuration for creating Cloud object
type CloudConfig struct {
	ClusterName string

	lbListenerSubnetID string
	lbTgNetworkID      string
	FolderID           string
	LocalRegion        string
	LocalZone          string
	RouteTableID       string

	InternalNetworkIDsSet map[string]struct{}
	ExternalNetworkIDsSet map[string]struct{}

	Credentials ycsdk.Credentials
}

// Cloud is an implementation of cloudprovider.Interface for Yandex.Cloud
type Cloud struct {
	yandexService         *yapi.YandexCloudAPI
	nodeTargetGroupSyncer *NodeTargetGroupSyncer
	config                CloudConfig

	nodeLister v1.NodeLister
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			config, err := NewCloudConfig()
			if err != nil {
				return nil, err
			}

			api, err := yapi.NewYandexCloudAPI(config.Credentials, config.LocalRegion, config.FolderID)
			if err != nil {
				return nil, err
			}

			return NewCloud(*config, api), nil
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

	cloudConfig.ClusterName = os.Getenv(envClusterName)
	if len(cloudConfig.ClusterName) == 0 {
		log.Fatalf("%q env is required", envClusterName)
	}

	cloudConfig.RouteTableID = os.Getenv(envRouteTableID)

	cloudConfig.lbListenerSubnetID = os.Getenv(envLbListenerSubnetID)

	cloudConfig.lbTgNetworkID = os.Getenv(envLbTgNetworkID)
	if len(cloudConfig.lbTgNetworkID) == 0 {
		log.Fatalf("%q env is required", envLbTgNetworkID)
	}

	cloudConfig.InternalNetworkIDsSet = make(map[string]struct{})
	cloudConfig.ExternalNetworkIDsSet = make(map[string]struct{})

	if len(os.Getenv(envInternalNetworkIDs)) > 0 {
		for _, networkID := range strings.Split(os.Getenv(envInternalNetworkIDs), ",") {
			cloudConfig.InternalNetworkIDsSet[networkID] = struct{}{}
		}
	}

	if len(os.Getenv(envExternalNetworkIDs)) > 0 {
		for _, networkID := range strings.Split(os.Getenv(envExternalNetworkIDs), ",") {
			cloudConfig.ExternalNetworkIDsSet[networkID] = struct{}{}
		}
	}

	// Retrieve LocalZone
	localZone := "ru-central1-b"
	cloudConfig.LocalZone = localZone
	cloudConfig.LocalRegion, err = GetRegion(localZone)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get region from zone: %s", localZone)
	}

	return cloudConfig, nil
}

// NewCloud creates a new instance of Cloud object
func NewCloud(config CloudConfig, api *yapi.YandexCloudAPI) *Cloud {
	return &Cloud{
		yandexService: api,
		config:        config,
	}
}

// Initialize passes a Kubernetes clientBuilder interface to the cloud provider
func (yc *Cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	clientset := clientBuilder.ClientOrDie("cloud-controller-manager")

	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	serviceInformer := informerFactory.Core().V1().Services()
	nodeInformer := informerFactory.Core().V1().Nodes()

	yc.nodeTargetGroupSyncer = &NodeTargetGroupSyncer{
		cloud:            yc,
		serviceLister:    serviceInformer.Lister(),
		lastVisitedNodes: mapset.NewSet(),
	}

	yc.nodeLister = nodeInformer.Lister()

	go serviceInformer.Informer().Run(stop)

	if !cache.WaitForCacheSync(stop, serviceInformer.Informer().HasSynced) {
		log.Printf("Timed out waiting for caches to sync")
	}
}

// LoadBalancer returns a balancer interface if supported.
func (yc *Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return yc, true
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
	if len(yc.config.RouteTableID) == 0 {
		return nil, false
	}

	return yc, true
}

// ProviderName returns the cloud provider ID.
func (yc *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if the cluster has a clusterID
func (yc *Cloud) HasClusterID() bool {
	return true
}

// InstancesV2 returns a InstancesV2 interface if supported
// TODO (zuzzas): implement once it's stable enough (starting in v0.20.0)
func (yc *Cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}
