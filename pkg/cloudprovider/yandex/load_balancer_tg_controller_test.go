package yandex

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
