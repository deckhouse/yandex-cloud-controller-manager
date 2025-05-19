package yandex

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"k8s.io/klog/v2"

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
)

type NodeTargetGroupSyncer struct {
	// TODO: refactor cloud out of here
	cloud *Cloud

	lastVisitedNodes mapset.Set
	serviceLister    corev1listers.ServiceLister

	tgSyncLock sync.Mutex
}

func (ntgs *NodeTargetGroupSyncer) SyncTGs(ctx context.Context, nodes []*corev1.Node) error {
	ntgs.tgSyncLock.Lock()
	defer ntgs.tgSyncLock.Unlock()

	services, err := ntgs.serviceLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Services from an internal Indexer: %s", err)
	}

	var activeLoadBalancerServicesExist bool
	for _, service := range services {
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer && service.ObjectMeta.DeletionTimestamp == nil {
			activeLoadBalancerServicesExist = true
			break
		}
	}
	// If no nodes passed seems we are called from the LoadBalancer delete function.
	// And if no LoadBalancer Services are left in the cluster – we should clean up target groups from the cloud.
	if len(nodes) == 0 && !activeLoadBalancerServicesExist {
		return ntgs.cleanUpTargetGroups(ctx)
	}

	err = ntgs.synchronizeNodesWithTargetGroups(ctx, nodes)
	if err != nil {
		return err
	}

	return nil
}

type tgNameToTargetMap map[string][]*loadbalancer.Target

func fromNodeToInterfaceSlice(nodes []*corev1.Node) (ret []interface{}) {
	for _, node := range nodes {
		ret = append(ret, node.Name)
	}

	return
}

func (ntgs *NodeTargetGroupSyncer) cleanUpTargetGroups(ctx context.Context) error {
	tgs, err := ntgs.cloud.yandexService.LbSvc.GetTGsByClusterName(ctx, ntgs.cloud.config.ClusterName)
	if err != nil {
		return err
	}

	wg, ctx := errgroup.WithContext(ctx)
	for _, tg := range tgs {
		tg := tg
		wg.Go(func() error {
			return ntgs.cloud.yandexService.LbSvc.RemoveTGByID(ctx, tg.Id)
		})
	}

	if err = wg.Wait(); err != nil {
		return err
	}

	ntgs.lastVisitedNodes.Clear()

	return nil
}

type instanceWithNodeInfo struct {
	Instance *compute.Instance
	Node     *corev1.Node
}

func (ntgs *NodeTargetGroupSyncer) synchronizeNodesWithTargetGroups(ctx context.Context, nodes []*corev1.Node) error {
	if len(nodes) == 0 {
		klog.Info("no nodes to synchronize TGs with, skipping...")
		return nil
	}

	newSet := mapset.NewSetFromSlice(fromNodeToInterfaceSlice(nodes))
	if ntgs.lastVisitedNodes.Equal(newSet) {
		return nil
	}

	// TODO: speed up by not performing individual lookups
	var instances []*instanceWithNodeInfo
	for _, node := range nodes {
		if node.Spec.ProviderID == "" {
			return errors.Errorf("node %s ProviderID is empty", node.Name)
		}

		if !(strings.Contains(node.Spec.ProviderID, "yandex")) {
			log.Printf("node %s ProviderID is not yandex (%s), skipping", node.Name, node.Spec.ProviderID)
			continue
		}

		nodeName := MapNodeNameToInstanceName(types.NodeName(node.Name))
		log.Printf("Finding Instance by Folder %q and Name %q", ntgs.cloud.config.FolderID, nodeName)
		instance, err := ntgs.cloud.yandexService.ComputeSvc.FindInstanceByName(ctx, nodeName)
		if err != nil || instance == nil {
			return fmt.Errorf("failed to find Instance by its name: %s", err)
		}

		instances = append(instances, &instanceWithNodeInfo{Instance: instance, Node: node})
	}

	mapping, err := ntgs.constructTgNameToTargetMap(ctx, instances)
	if err != nil {
		return fmt.Errorf("failed to construct tgNameToTargetMap: %s", err)
	}

	for tgName, targets := range mapping {
		_, err := ntgs.cloud.yandexService.LbSvc.CreateOrUpdateTG(ctx, tgName, targets)
		if err != nil {
			return err
		}
	}

	ntgs.lastVisitedNodes = newSet

	return nil
}

func (ntgs *NodeTargetGroupSyncer) constructTgNameToTargetMap(ctx context.Context, instances []*instanceWithNodeInfo) (tgNameToTargetMap, error) {
	mapping := make(tgNameToTargetMap)

	// TODO: Implement simple caching mechanism for subnet-VPC membership lookups
	for _, instance := range instances {
		for _, iface := range instance.Instance.NetworkInterfaces {
			subnetInfo, err := ntgs.cloud.yandexService.VPCSvc.SubnetSvc.Get(ctx, &vpc.GetSubnetRequest{SubnetId: iface.SubnetId})
			if err != nil {
				return nil, errors.WithStack(err)
			}

			key := ntgs.cloud.config.ClusterName + subnetInfo.NetworkId
			if v, ok := instance.Node.Annotations[customTargetGroupNamePrefixAnnotation]; ok {
				key = truncateAnnotationValue(v) + key
			}
			mapping[key] = append(mapping[key], &loadbalancer.Target{
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

func truncateAnnotationValue(value string) string {
	// maximum length of annotation values should not exceed 63 - length of cluster uuid(26 symbols) - length of network id(21)
	if len(value) > 36 {
		log.Printf("annotation '%s' length should be less than 36 characters, truncate it", value)
		value = value[:36]
	}
	return value
}
