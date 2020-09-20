package yandex

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1listers "k8s.io/client-go/listers/core/v1"

	mapset "github.com/deckarep/golang-set"

	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type NodeTargetGroupSyncer struct {
	// TODO: refactor cloud out of here
	cloud *Cloud

	latestVisitedNodes mapset.Set

	kubeclientset kubernetes.Interface
	nodeLister    corev1listers.NodeLister
	serviceLister corev1listers.ServiceLister
	nodeSynced    bool
	workqueue     workqueue.RateLimitingInterface
	recorder      record.EventRecorder
}

func (c *NodeTargetGroupSyncer) enqueueNode(obj interface{}) {
	var (
		key string
		err error
	)

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

func (c *NodeTargetGroupSyncer) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeTargetGroupSyncer) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)

		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *NodeTargetGroupSyncer) syncHandler() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	services, err := c.serviceLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to List node from internal Indexer: %s", err)
	}
	if len(services) == 0 {
		return c.cloud.CleanUpTargetGroups(ctx)
	}

	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to List node from internal Indexer: %s", err)
	}
	if len(nodes) == 0 {
		log.Println("no Nodes detected, cannot sync")
		return nil
	}

	err = c.cloud.SynchronizeNodesWithTargetGroups(ctx, nodes)
	if err != nil {
		return err
	}

	return nil
}

type networkIdToTargetMap map[string][]*loadbalancer.Target

func fromNodeToInterfaceSlice(nodes []*corev1.Node) (ret []interface{}) {
	for _, node := range nodes {
		ret = append(ret, node.Name)
	}

	return
}

func (yc *Cloud) CleanUpTargetGroups(ctx context.Context) error {
	tgs, err := yc.yandexService.LbSvc.GetTGsByClusterName(ctx, yc.config.ClusterName)
	if err != nil {
		return err
	}

	wg, ctx := errgroup.WithContext(ctx)
	for _, tg := range tgs {
		wg.Go(func() error {
			return yc.yandexService.LbSvc.RemoveTGByID(ctx, tg.Id)
		})
	}

	return wg.Wait()
}

func (yc *Cloud) SynchronizeNodesWithTargetGroups(ctx context.Context, nodes []*corev1.Node) error {
	newSet := mapset.NewSetFromSlice(fromNodeToInterfaceSlice(nodes))
	if yc.nodeTargetGroupSyncer.latestVisitedNodes.Equal(newSet) {
		return nil
	}

	// TODO: speed up by not performing individual lookups
	var instances []*compute.Instance
	for _, node := range nodes {
		nodeName := MapNodeNameToInstanceName(types.NodeName(node.Name))
		log.Printf("Finding Instance by Folder %q and Name %q", yc.config.FolderID, nodeName)
		instance, err := yc.yandexService.ComputeSvc.FindInstanceByName(ctx, nodeName)
		if err != nil || instance == nil {
			return fmt.Errorf("failed to find Instance by its name: %s", err)
		}

		instances = append(instances, instance)
	}

	mapping, err := yc.constructNetworkIdToTargetMap(ctx, instances)
	if err != nil {
		return fmt.Errorf("failed to construct NetworkIdToTargetMap: %s", err)
	}

	for networkID, targets := range mapping {
		_, err := yc.yandexService.LbSvc.CreateOrUpdateTG(ctx, yc.config.ClusterName+networkID, targets)
		if err != nil {
			return err
		}
	}

	yc.nodeTargetGroupSyncer.latestVisitedNodes = newSet

	return nil
}

func (yc *Cloud) constructNetworkIdToTargetMap(ctx context.Context, instances []*compute.Instance) (networkIdToTargetMap, error) {
	mapping := make(networkIdToTargetMap)

	// TODO: Implement simple caching mechanism for subnet-VPC membership lookups
	for _, instance := range instances {
		for _, iface := range instance.NetworkInterfaces {
			subnetInfo, err := yc.yandexService.VPCSvc.SubnetSvc.Get(ctx, &vpc.GetSubnetRequest{SubnetId: iface.SubnetId})
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
