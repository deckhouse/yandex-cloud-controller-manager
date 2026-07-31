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
