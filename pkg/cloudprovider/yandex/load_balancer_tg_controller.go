package yandex

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"k8s.io/klog/v2"

	"golang.org/x/sync/errgroup"

	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	corev1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	corev1listers "k8s.io/client-go/listers/core/v1"

	mapset "github.com/deckarep/golang-set"

	"k8s.io/apimachinery/pkg/labels"
)

type NodeTargetGroupSyncer struct {
	// TODO: refactor cloud out of here
	cloud *Cloud

	lastVisitedNodes mapset.Set
	lastSkippedNodes string
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
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer && service.DeletionTimestamp == nil {
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

func nodeTargetGroupSyncState(nodes []*corev1.Node) (ret []interface{}) {
	for _, node := range nodes {
		ret = append(ret, fmt.Sprintf(
			"%s\x00%s\x00%s",
			node.Name,
			node.Spec.ProviderID,
			node.Annotations[customTargetGroupNamePrefixAnnotation],
		))
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

	// Both caches describe the target groups that were just removed, so neither may outlive them:
	// keeping lastSkippedNodes would swallow the warning about a Node left out of the target groups
	// the next time a LoadBalancer Service appears.
	ntgs.lastVisitedNodes.Clear()
	ntgs.lastSkippedNodes = ""

	return nil
}

type instanceWithNodeInfo struct {
	Instance *compute.Instance
	Node     *corev1.Node
}

func hasTaint(node *corev1.Node, key string) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == key {
			return true
		}
	}

	return false
}

// partitionNodesByProviderID splits Nodes into the ones that can be placed into a target group
// and human-readable reasons for the ones that cannot. Nodes without a ProviderID are not an
// error: a freshly registered Node has no ProviderID until the node controller assigns one, and
// a cluster may legitimately mix cloud Nodes with static ones that never get a Yandex ProviderID.
//
// This is a different notion of eligibility from nodeEligibleForLoadBalancer, which filters on
// Node state – deletion, exclusion label, autoscaler taint – rather than on the ProviderID.
//
// Filtering is silent: callers decide what to report, so that a steady-state cluster does not
// repeat the same per-Node messages at default verbosity on every synchronization.
func partitionNodesByProviderID(nodes []*corev1.Node) (yandexNodes []*corev1.Node, skipped []string) {
	yandexNodes = make([]*corev1.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Spec.ProviderID == "" {
			// Two very different causes hide behind an empty ProviderID, and the taint tells them
			// apart for free. With the taint, kubelet did hand the Node to a cloud provider and
			// cloud-node has simply not initialized it yet, which normally takes seconds. Without
			// it, the Node was never offered to a cloud provider at all: either a genuine static
			// Node, or a cloud VM whose kubelet is missing --cloud-provider=external – and in that
			// second case the Node stays out of the target groups until that is fixed, so saying
			// which of the two it is turns a silent capacity loss into something actionable.
			reason := "awaiting cloud-node initialization"
			if !hasTaint(node, cloudproviderapi.TaintExternalCloudProvider) {
				reason = fmt.Sprintf("no %s taint, never handed to a cloud provider",
					cloudproviderapi.TaintExternalCloudProvider)
			}
			skipped = append(skipped, fmt.Sprintf("%s (%s)", node.Name, reason))

			continue
		}

		// ParseProviderID anchors on the yandex:// scheme, unlike a substring match, which would
		// also accept a foreign ProviderID that merely mentions the provider name. Its result is
		// what the Instance lookup consumes, so an unparsable value has to be rejected here.
		if _, _, err := ParseProviderID(node.Spec.ProviderID); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", node.Name, err))
			continue
		}

		yandexNodes = append(yandexNodes, node)
	}

	return yandexNodes, skipped
}

// newlySkippedNodes returns the Nodes left out of the target groups rendered for a log line, or an
// empty string when there is nothing new to report. Skipped Nodes have to be visible at default
// verbosity – an operator must be able to see that a Node dropped out of the target groups – but a
// cluster with static Nodes would otherwise repeat the same warning on every synchronization.
//
// Sorting does double duty: Node order from the informer is not stable, so unsorted reasons would
// compare unequal at random, and it keeps the reported line itself stable between reports. The
// argument is a freshly built slice, so sorting it in place is safe.
func (ntgs *NodeTargetGroupSyncer) newlySkippedNodes(skipped []string) string {
	sort.Strings(skipped)

	current := strings.Join(skipped, "; ")
	if current == ntgs.lastSkippedNodes {
		return ""
	}
	ntgs.lastSkippedNodes = current

	return current
}

func (ntgs *NodeTargetGroupSyncer) synchronizeNodesWithTargetGroups(ctx context.Context, nodes []*corev1.Node) error {
	if len(nodes) == 0 {
		klog.Info("no nodes to synchronize TGs with, skipping...")
		return nil
	}

	yandexNodes, skippedNodes := partitionNodesByProviderID(nodes)

	// Reported before the lastVisitedNodes check below: a skipped Node never reaches that cache,
	// so gating this on a changed Node set would hide it entirely.
	if reasons := ntgs.newlySkippedNodes(skippedNodes); reasons != "" {
		klog.Warningf("skipping %d of %d Nodes during TG synchronization: %s",
			len(skippedNodes), len(nodes), reasons)
	}

	if len(yandexNodes) == 0 {
		klog.Warningf("none of %d Nodes have a valid Yandex ProviderID, skipping TG synchronization", len(nodes))
		return nil
	}

	newSet := mapset.NewSetFromSlice(nodeTargetGroupSyncState(yandexNodes))
	if ntgs.lastVisitedNodes.Equal(newSet) {
		return nil
	}

	var instances []*instanceWithNodeInfo
	for _, node := range yandexNodes {
		// getInstanceByProviderID resolves a modern yandex://<id> ProviderID with a single Get by
		// ID instead of listing the folder by Node name, which also means a Node is never matched
		// to an unrelated Instance that happens to share its name.
		instance, err := ntgs.cloud.getInstanceByProviderID(ctx, node.Spec.ProviderID)
		if err != nil {
			// A deleted Instance must not abort synchronization for every other Node, the same way
			// an empty ProviderID must not. Network and server-side failures still do. The state is
			// self-clearing: cloud-node-lifecycle removes Nodes whose Instance no longer exists.
			if errors.Is(err, cloudprovider.InstanceNotFound) {
				klog.Warningf("Instance for Node %s (%s) no longer exists, leaving it out of the target groups",
					node.Name, node.Spec.ProviderID)
				continue
			}
			return fmt.Errorf("failed to find Instance for Node %s: %w", node.Name, err)
		}

		instances = append(instances, &instanceWithNodeInfo{Instance: instance, Node: node})
	}

	mapping, err := ntgs.constructTgNameToTargetMap(ctx, instances)
	if err != nil {
		return fmt.Errorf("failed to construct tgNameToTargetMap: %s", err)
	}

	if err := ntgs.cloud.yandexService.LbSvc.ReconcileTargetGroups(ctx, ntgs.cloud.config.ClusterName, mapping); err != nil {
		return err
	}

	// The set that was achieved, not the one that was desired: a Node left out because its Instance
	// is gone has to be retried on the next synchronization rather than pinned until an unrelated
	// change makes the desired set differ again.
	syncedNodes := make([]*corev1.Node, 0, len(instances))
	for _, instance := range instances {
		syncedNodes = append(syncedNodes, instance.Node)
	}
	ntgs.lastVisitedNodes = mapset.NewSetFromSlice(nodeTargetGroupSyncState(syncedNodes))

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
