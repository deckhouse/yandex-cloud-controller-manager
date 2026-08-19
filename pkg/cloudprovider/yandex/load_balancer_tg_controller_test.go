package yandex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	cloudproviderapi "k8s.io/cloud-provider/api"

	"github.com/deckhouse/yandex-cloud-controller-manager/pkg/yapi"

	mapset "github.com/deckarep/golang-set"
)

func TestNodeTargetGroupSyncState(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Spec:       corev1.NodeSpec{ProviderID: "yandex://instance-1"},
	}

	initial := mapset.NewSetFromSlice(nodeTargetGroupSyncState([]*corev1.Node{node}))

	node.Annotations = map[string]string{customTargetGroupNamePrefixAnnotation: "frontend"}
	withAnnotation := mapset.NewSetFromSlice(nodeTargetGroupSyncState([]*corev1.Node{node}))
	if initial.Equal(withAnnotation) {
		t.Fatal("target group annotation change must invalidate the node sync state")
	}

	node.Spec.ProviderID = "yandex://instance-2"
	withNewProviderID := mapset.NewSetFromSlice(nodeTargetGroupSyncState([]*corev1.Node{node}))
	if withAnnotation.Equal(withNewProviderID) {
		t.Fatal("provider ID change must invalidate the node sync state")
	}
}

func TestNodeTargetGroupAnnotationChanged(t *testing.T) {
	oldNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "node-1",
		Annotations: map[string]string{customTargetGroupNamePrefixAnnotation: "frontend"},
	}}

	unrelatedUpdate := oldNode.DeepCopy()
	unrelatedUpdate.Labels = map[string]string{"example.com/test": "value"}
	if nodeTargetGroupAnnotationChanged(oldNode, unrelatedUpdate) {
		t.Fatal("unrelated Node update must not trigger target group synchronization")
	}

	annotationUpdate := oldNode.DeepCopy()
	delete(annotationUpdate.Annotations, customTargetGroupNamePrefixAnnotation)
	if !nodeTargetGroupAnnotationChanged(oldNode, annotationUpdate) {
		t.Fatal("target group annotation update must trigger synchronization")
	}
}

func TestNodeEligibleForLoadBalancer(t *testing.T) {
	eligible := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "eligible"}}
	if !nodeEligibleForLoadBalancer(eligible) {
		t.Fatal("ordinary Node must be eligible")
	}

	excluded := eligible.DeepCopy()
	excluded.Labels = map[string]string{corev1.LabelNodeExcludeBalancers: "true"}
	if nodeEligibleForLoadBalancer(excluded) {
		t.Fatal("Node excluded from external load balancers must not be eligible")
	}

	tainted := eligible.DeepCopy()
	tainted.Spec.Taints = []corev1.Taint{{Key: "ToBeDeletedByClusterAutoscaler"}}
	if nodeEligibleForLoadBalancer(tainted) {
		t.Fatal("Node marked for deletion by cluster autoscaler must not be eligible")
	}
}

func newTestTargetGroupSyncQueue() workqueue.TypedRateLimitingInterface[string] {
	return workqueue.NewTypedRateLimitingQueue(
		workqueue.NewTypedItemExponentialFailureRateLimiter[string](time.Nanosecond, time.Nanosecond),
	)
}

func TestTargetGroupSyncWorkerRetriesUntilSuccess(t *testing.T) {
	queue := newTestTargetGroupSyncQueue()
	queue.Add(targetGroupSyncKey)

	attempts := 0
	runTargetGroupSyncWorker(context.Background(), queue, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		queue.ShutDown()
		return nil
	})

	if attempts != 3 {
		t.Fatalf("sync attempts = %d, want 3", attempts)
	}
}

func TestTargetGroupSyncWorkerCoalescesEvents(t *testing.T) {
	queue := newTestTargetGroupSyncQueue()
	queue.Add(targetGroupSyncKey)

	attempts := 0
	runTargetGroupSyncWorker(context.Background(), queue, func(context.Context) error {
		attempts++
		if attempts == 1 {
			for i := 0; i < 10; i++ {
				queue.Add(targetGroupSyncKey)
			}
			return nil
		}
		queue.ShutDown()
		return nil
	})

	if attempts != 2 {
		t.Fatalf("sync attempts = %d, want 2", attempts)
	}
}

func TestTargetGroupSyncWorkerStopsAfterMaxRetries(t *testing.T) {
	queue := newTestTargetGroupSyncQueue()
	queue.Add(targetGroupSyncKey)

	attempts := 0
	runTargetGroupSyncWorker(context.Background(), queue, func(context.Context) error {
		attempts++
		if attempts == targetGroupSyncMaxRetries {
			queue.ShutDown()
		}
		return errors.New("persistent error")
	})

	if attempts != targetGroupSyncMaxRetries {
		t.Fatalf("sync attempts = %d, want %d", attempts, targetGroupSyncMaxRetries)
	}
}

func TestTargetGroupSyncWorkerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := newTestTargetGroupSyncQueue()
	context.AfterFunc(ctx, queue.ShutDown)
	queue.Add(targetGroupSyncKey)

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTargetGroupSyncWorker(ctx, queue, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("target group sync worker did not stop after context cancellation")
	}
}

func TestSynchronizeNodesWithTargetGroupsSkipsWhenAllProviderIDsAreEmpty(t *testing.T) {
	lastVisitedNodes := mapset.NewSet("previous-node")
	syncer := &NodeTargetGroupSyncer{lastVisitedNodes: lastVisitedNodes}
	nodes := []*corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "stale-node-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "stale-node-2"}},
	}

	if err := syncer.synchronizeNodesWithTargetGroups(context.Background(), nodes); err != nil {
		t.Fatalf("expected synchronization to be skipped, got error: %v", err)
	}
	if !syncer.lastVisitedNodes.Equal(lastVisitedNodes) {
		t.Fatal("expected last visited nodes cache to remain unchanged")
	}
}

func TestPartitionNodesByProviderIDKeepsYandexNodesAlongsideSkippedOnes(t *testing.T) {
	// A static Node deliberately comes first: an implementation that aborts on the first
	// unusable Node instead of skipping it would drop the cloud Nodes that follow.
	nodes := []*corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "static-node"}},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cloud-node-1"},
			Spec:       corev1.NodeSpec{ProviderID: "yandex://instance-1"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "foreign-node"},
			Spec:       corev1.NodeSpec{ProviderID: "aws:///eu-central-1a/i-1"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "deprecated-node"},
			Spec:       corev1.NodeSpec{ProviderID: "yandex://folder/ru-central1-a/instance-3"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cloud-node-2"},
			Spec:       corev1.NodeSpec{ProviderID: "yandex://instance-2"},
		},
	}

	yandexNodes, skipped := partitionNodesByProviderID(nodes)

	yandexNames := make([]string, 0, len(yandexNodes))
	for _, node := range yandexNodes {
		yandexNames = append(yandexNames, node.Name)
	}
	expected := []string{"cloud-node-1", "deprecated-node", "cloud-node-2"}
	if !reflect.DeepEqual(yandexNames, expected) {
		t.Fatalf("expected Yandex Nodes %v, got %v", expected, yandexNames)
	}

	if len(skipped) != 2 {
		t.Fatalf("expected 2 skipped Nodes, got %d: %v", len(skipped), skipped)
	}
	for _, want := range []string{"static-node", "foreign-node"} {
		found := false
		for _, reason := range skipped {
			if strings.Contains(reason, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be reported as skipped, got %v", want, skipped)
		}
	}
}

func TestPartitionNodesByProviderIDRejectsForeignProviderIDMentioningYandex(t *testing.T) {
	// A substring match on "yandex" would accept this Node. Since the Instance is then resolved
	// by Node name, an unrelated VM sharing that name in the Yandex folder would be attached to
	// the cluster's target group.
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "impostor-node"},
			Spec:       corev1.NodeSpec{ProviderID: "aws:///eu-central-1a/i-0yandex123"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "malformed-node"},
			Spec:       corev1.NodeSpec{ProviderID: "yandex://"},
		},
	}

	yandexNodes, skipped := partitionNodesByProviderID(nodes)
	if len(yandexNodes) != 0 {
		t.Fatalf("expected no Yandex Nodes, got %v", yandexNodes)
	}
	if len(skipped) != 2 {
		t.Fatalf("expected both Nodes to be reported as skipped, got %v", skipped)
	}
}

func TestPartitionNodesByProviderIDReturnsNoNodesWhenProviderIDsAreEmpty(t *testing.T) {
	nodes := []*corev1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "stale-node-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "stale-node-2"}},
	}

	yandexNodes, skipped := partitionNodesByProviderID(nodes)
	if len(yandexNodes) != 0 {
		t.Fatalf("expected no Yandex Nodes, got %d", len(yandexNodes))
	}
	if len(skipped) != 2 {
		t.Fatalf("expected both Nodes to be reported as skipped, got %v", skipped)
	}
}

func TestNewlySkippedNodesReportsOnlyOnChange(t *testing.T) {
	syncer := &NodeTargetGroupSyncer{}
	first := []string{"static-2 (ProviderID is empty)", "static-1 (ProviderID is empty)"}

	reported := syncer.newlySkippedNodes(first)
	if reported == "" {
		t.Fatal("the first observation of skipped Nodes must be reported")
	}
	// Reported reasons are sorted, so the log line stays stable between reports even though the
	// informer hands Nodes over in no particular order.
	if reported != "static-1 (ProviderID is empty); static-2 (ProviderID is empty)" {
		t.Fatalf("expected the reported reasons to be sorted, got %q", reported)
	}

	// Same Nodes in a different order are the same set and must not be reported again.
	if again := syncer.newlySkippedNodes([]string{first[1], first[0]}); again != "" {
		t.Fatalf("an unchanged set of skipped Nodes must not be reported again, got %q", again)
	}

	grown := []string{first[0], first[1], "static-3 (ProviderID is empty)"}
	if syncer.newlySkippedNodes(grown) == "" {
		t.Fatal("a newly skipped Node must be reported")
	}

	// Nothing skipped any more: the state has to be cleared so a later skip is reported again,
	// while the transition itself has nothing to log.
	if empty := syncer.newlySkippedNodes(nil); empty != "" {
		t.Fatalf("an empty set of skipped Nodes has nothing to report, got %q", empty)
	}
	if syncer.newlySkippedNodes(grown) == "" {
		t.Fatal("skipped Nodes returning after an empty set must be reported again")
	}
}

func TestPartitionNodesByProviderIDDistinguishesUninitializedFromStaticNodes(t *testing.T) {
	// PR #195 hit this in production: a Node reaching reconciliation with an empty ProviderID.
	// Whose problem it is depends entirely on the taint, so the reasons have to differ.
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "awaiting-init"},
			Spec: corev1.NodeSpec{Taints: []corev1.Taint{{
				Key:    cloudproviderapi.TaintExternalCloudProvider,
				Effect: corev1.TaintEffectNoSchedule,
			}}},
		},
		{ObjectMeta: metav1.ObjectMeta{Name: "never-offered"}},
	}

	yandexNodes, skipped := partitionNodesByProviderID(nodes)
	if len(yandexNodes) != 0 {
		t.Fatalf("expected no Yandex Nodes, got %d", len(yandexNodes))
	}
	if len(skipped) != 2 {
		t.Fatalf("expected both Nodes to be reported as skipped, got %v", skipped)
	}

	if !strings.Contains(skipped[0], "awaiting cloud-node initialization") {
		t.Errorf("a Node carrying the %s taint must be reported as awaiting initialization, got %q",
			cloudproviderapi.TaintExternalCloudProvider, skipped[0])
	}
	if !strings.Contains(skipped[1], "never handed to a cloud provider") {
		t.Errorf("a Node without the taint must be reported as never handed to a cloud provider, got %q", skipped[1])
	}
	// The two causes need different remediation, so they must not collapse into one message.
	if skipped[0] == skipped[1] {
		t.Fatal("the two causes of an empty ProviderID must produce different reasons")
	}
}

// fakeEmptyTargetGroupClient reports no target groups, which is enough to walk
// cleanUpTargetGroups down to the cache reset without any operations to wait on.
type fakeEmptyTargetGroupClient struct {
	loadbalancer.TargetGroupServiceClient
}

func (fakeEmptyTargetGroupClient) List(_ context.Context, _ *loadbalancer.ListTargetGroupsRequest, _ ...grpc.CallOption) (*loadbalancer.ListTargetGroupsResponse, error) {
	return &loadbalancer.ListTargetGroupsResponse{}, nil
}

func TestCleanUpTargetGroupsResetsSkippedNodeReport(t *testing.T) {
	// Both caches describe target groups that cleanUpTargetGroups has just removed. If the skipped
	// Node report survives them, the next LoadBalancer Service is created without any warning that
	// a Node is being left out of its target groups.
	syncer := &NodeTargetGroupSyncer{
		cloud: &Cloud{
			config: CloudConfig{ClusterName: "test-cluster"},
			yandexService: &yapi.YandexCloudAPI{
				LbSvc: yapi.NewLoadBalancerService(nil, fakeEmptyTargetGroupClient{}, &yapi.CloudContext{FolderID: "test-folder"}),
			},
		},
		lastVisitedNodes: mapset.NewSet(),
	}

	skipped := []string{"static-1 (never handed to a cloud provider)"}
	if syncer.newlySkippedNodes(skipped) == "" {
		t.Fatal("the first observation of skipped Nodes must be reported")
	}
	syncer.lastVisitedNodes.Add("stale")

	if err := syncer.cleanUpTargetGroups(context.Background()); err != nil {
		t.Fatalf("cleanUpTargetGroups: %v", err)
	}

	if syncer.lastVisitedNodes.Cardinality() != 0 {
		t.Errorf("expected the visited Node cache to be cleared, got %v", syncer.lastVisitedNodes)
	}
	if syncer.newlySkippedNodes(skipped) == "" {
		t.Error("after the target groups are torn down the same skipped Nodes must be reported again")
	}
}
