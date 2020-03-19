package yandex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/flant/yandex-cloud-controller-manager/pkg/yapi"

	"golang.org/x/time/rate"

	mapset "github.com/deckarep/golang-set"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/tools/record"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"

	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

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

	InternalNetworkIDsSet map[string]struct{}
	ExternalNetworkIDsSet map[string]struct{}

	Credentials ycsdk.Credentials
}

// Cloud is an implementation of cloudprovider.Interface for Yandex.Cloud
type Cloud struct {
	api                   *yapi.YandexCloudAPI
	nodeTargetGroupSyncer *NodeTargetGroupSyncer
	config                CloudConfig
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
	localZone, err := metadata.GetZone()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get Zone from instance metadata")
	}
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
		api:    api,
		config: config,
	}
}

// Initialize passes a Kubernetes clientBuilder interface to the cloud provider
func (yc *Cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	clientset := clientBuilder.ClientOrDie("cloud-controller-manager")

	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	nodeInformer := informerFactory.Core().V1().Nodes()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "yandex-cloud-controller-manager"})

	yc.nodeTargetGroupSyncer = &NodeTargetGroupSyncer{
		cloud:         yc,
		kubeclientset: clientset,
		nodeLister:    nodeInformer.Lister(),
		workqueue: workqueue.NewRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 60*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		)),
		recorder:           recorder,
		latestVisitedNodes: mapset.NewSet(),
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    yc.nodeTargetGroupSyncer.enqueueNode,
		UpdateFunc: func(oldObj, newObj interface{}) { return },
		DeleteFunc: yc.nodeTargetGroupSyncer.enqueueNode,
	})

	go nodeInformer.Informer().Run(stop)

	if !cache.WaitForCacheSync(stop, nodeInformer.Informer().HasSynced) {
		log.Printf("Timed out waiting for caches to sync")
	}

	go wait.Until(yc.nodeTargetGroupSyncer.runWorker, time.Second, stop)
}

type networkIdToTargetMap map[string][]*loadbalancer.Target

func fromNodeToInterfaceSlice(nodes []*corev1.Node) (ret []interface{}) {
	for _, node := range nodes {
		ret = append(ret, node.Name)
	}

	return
}

func (yc *Cloud) SynchronizeNodesWithTargetGroups(ctx context.Context, nodes []*corev1.Node) error {
	newSet := mapset.NewSetFromSlice(fromNodeToInterfaceSlice(nodes))
	if yc.nodeTargetGroupSyncer.latestVisitedNodes.Equal(newSet) {
		return nil
	}

	// TODO: speed up using goroutines?
	var instances []*compute.Instance
	for _, node := range nodes {
		nodeName := MapNodeNameToInstanceName(types.NodeName(node.Name))
		log.Printf("Finding Instance by Folder %q and Name %q", yc.config.FolderID, nodeName)
		instance, err := yc.api.FindInstanceByFolderAndName(ctx, yc.config.FolderID, nodeName)
		if err != nil {
			return fmt.Errorf("failed to find Instance by its name: %s", err)
		}

		instances = append(instances, instance)
	}

	mapping, err := yc.constructNetworkIdToTargetMap(ctx, instances)
	if err != nil {
		return fmt.Errorf("failed to construct NetworkIdToTargetMap: %s", err)
	}

	for networkID, targets := range mapping {
		// TODO: unique ClusterID
		_, err := yc.api.LbSvc.CreateOrUpdateTG(ctx, yc.config.ClusterName+networkID, targets)
		if err != nil {
			return err
		}
	}

	yc.nodeTargetGroupSyncer.latestVisitedNodes = newSet

	return nil
}

func (yc *Cloud) constructNetworkIdToTargetMap(ctx context.Context, instances []*compute.Instance) (networkIdToTargetMap, error) {
	sdk := yc.api.GetSDK()

	mapping := make(networkIdToTargetMap)

	// TODO: Implement simple caching mechanism for subnet-VPC membership lookups
	for _, instance := range instances {
		for _, iface := range instance.NetworkInterfaces {
			subnetInfo, err := sdk.VPC().Subnet().Get(ctx, &vpc.GetSubnetRequest{SubnetId: iface.SubnetId})
			if err != nil {
				return nil, errors.WithStack(err)
			}

			mapping[subnetInfo.NetworkId] = append(mapping[subnetInfo.NetworkId], &loadbalancer.Target{
				SubnetId: iface.SubnetId,
				Address:  iface.PrimaryV4Address.Address,
			})
		}
	}

	if len(mapping) == 0 {
		return nil, errors.New("no mappings found")
	}

	return mapping, nil
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
