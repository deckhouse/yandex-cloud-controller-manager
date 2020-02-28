package yandex

import (
	"context"
	"fmt"
	"log"
	"time"

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
