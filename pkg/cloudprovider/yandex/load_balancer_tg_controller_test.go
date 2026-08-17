package yandex

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

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
