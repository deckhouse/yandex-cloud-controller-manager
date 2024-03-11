package yapi

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/apimachinery/pkg/util/sets"
)

type LoadBalancerService struct {
	cloudCtx *CloudContext

	LbSvc loadbalancer.NetworkLoadBalancerServiceClient
	TgSvc loadbalancer.TargetGroupServiceClient
}

func NewLoadBalancerService(lbSvc loadbalancer.NetworkLoadBalancerServiceClient, tgSvc loadbalancer.TargetGroupServiceClient,
	cloudCtx *CloudContext) *LoadBalancerService {

	return &LoadBalancerService{
		cloudCtx: cloudCtx,

		LbSvc: lbSvc,
		TgSvc: tgSvc,
	}
}

func logOperation(msgPrefix string, op *operation.Operation, err error) {
	id := "unknown"
	description := "no description"
	isDone := "unknown"
	str := "no str"
	if op != nil {
		id = op.Id
		description = op.Description
		if op.Done {
			isDone = "true"
		} else {
			isDone = "false"
		}
		str = op.String()
	}

	statusSuffix := "success"
	if err != nil {
		statusSuffix = fmt.Sprintf("got error: %v", err)
	}

	log.Printf("%s - %s.\nId: %s\nDescription: %s\nIsDone:%s\nString: %s", msgPrefix, statusSuffix, id, description, isDone, str)
}

func (ySvc *LoadBalancerService) CreateOrUpdateLB(ctx context.Context, name string, listenerSpec []*loadbalancer.ListenerSpec, attachedTGs []*loadbalancer.AttachedTargetGroup) (string, error) {
	log.Printf("CreateOrUpdateLB start")
	defer log.Printf("CreateOrUpdateLB finished")

	var nlbType = loadbalancer.NetworkLoadBalancer_EXTERNAL
	for _, listener := range listenerSpec {
		if _, ok := listener.Address.(*loadbalancer.ListenerSpec_InternalAddressSpec); ok {
			nlbType = loadbalancer.NetworkLoadBalancer_INTERNAL
			break
		}
	}

	log.Printf("Getting LB by name: %q", name)
	lb, err := ySvc.GetLbByName(ctx, name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Println("LB not found, creating new LB")
		} else {
			return "", err
		}
	}

	lbCreateRequest := &loadbalancer.CreateNetworkLoadBalancerRequest{
		FolderId:             ySvc.cloudCtx.FolderID,
		Name:                 name,
		RegionId:             ySvc.cloudCtx.RegionID,
		Type:                 nlbType,
		ListenerSpecs:        listenerSpec,
		AttachedTargetGroups: attachedTGs,
	}

	if lb == nil {
		log.Printf("Creating LoadBalancer: %+v", *lbCreateRequest)

		result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.Create(ctx, lbCreateRequest)
			logOperation("Creating LB", op, err)
			return op, err
		})
		if err != nil {
			return "", err
		}

		return result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address, nil
	}

	if lb != nil && shouldRecreate(lb, lbCreateRequest) {
		log.Printf("Re-creating LoadBalancer: %+v", *lbCreateRequest)

		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.Delete(ctx, &loadbalancer.DeleteNetworkLoadBalancerRequest{NetworkLoadBalancerId: lb.Id})
			logOperation("Deleting LB", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.Create(ctx, lbCreateRequest)
			logOperation("Creating LB", op, err)
			return op, err
		})
		if err != nil {
			return "", err
		}

		return result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address, nil
	}

	log.Printf("LB %q already exists, attempting an update\n", name)

	dirty := false

	listenersToAdd, listenersToRemove := diffListeners(listenerSpec, lb.Listeners)
	for _, listener := range listenersToRemove {
		req := &loadbalancer.RemoveNetworkLoadBalancerListenerRequest{
			NetworkLoadBalancerId: lb.Id,
			ListenerName:          listener.Name,
		}
		log.Printf("Removing Listener: %+v", *req)

		// todo(31337Ghost) it will be better to send requests concurrently
		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.RemoveListener(ctx, req)
			logOperation("Removing listener", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}
	for _, listener := range listenersToAdd {
		req := &loadbalancer.AddNetworkLoadBalancerListenerRequest{
			NetworkLoadBalancerId: lb.Id,
			ListenerSpec:          listener,
		}
		log.Printf("Adding Listener: %+v", *req)

		// todo(31337Ghost) it will be better to send requests concurrently
		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.AddListener(ctx, req)
			logOperation("Adding listener", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}

	tgsToAttach, tgsToDetach := diffAttachedTargetGroups(attachedTGs, lb.AttachedTargetGroups)
	for _, tg := range tgsToDetach {
		req := &loadbalancer.DetachNetworkLoadBalancerTargetGroupRequest{
			NetworkLoadBalancerId: lb.Id,
			TargetGroupId:         tg.TargetGroupId,
		}
		log.Printf("Detaching TargetGroup: %+v", *req)

		// todo(31337Ghost) it will be better to send requests concurrently
		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.DetachTargetGroup(ctx, req)
			logOperation("Detaching target group", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}
	for _, tg := range tgsToAttach {
		req := &loadbalancer.AttachNetworkLoadBalancerTargetGroupRequest{
			NetworkLoadBalancerId: lb.Id,
			AttachedTargetGroup:   tg,
		}
		log.Printf("Attaching TargetGroup: %+v", *req)

		// todo(31337Ghost) it will be better to send requests concurrently
		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.LbSvc.AttachTargetGroup(ctx, req)
			logOperation("Attaching target group", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}

	// Ensure that after all manipulations with LoadBalancer in the cloud it still exists.
	if dirty {
		log.Printf("Retrieving LoadBalancer %q after update", name)
		lb, err = ySvc.GetLbByName(ctx, name)
		if err != nil {
			return "", err
		}
	}

	return lb.Listeners[0].Address, nil
}

func (ySvc *LoadBalancerService) GetTGsByClusterName(ctx context.Context, clusterName string) (ret []*loadbalancer.TargetGroup, err error) {
	log.Printf("GetTGsByClusterName start")
	defer log.Printf("GetTGsByClusterName finished")

	result, err := ySvc.TgSvc.List(ctx, &loadbalancer.ListTargetGroupsRequest{
		FolderId: ySvc.cloudCtx.FolderID,
		// FIXME: properly implement iterator
		PageSize: 1000,
	})
	if err != nil {
		return nil, err
	}

	for _, tg := range result.TargetGroups {
		if strings.HasPrefix(tg.Name, clusterName) {
			ret = append(ret, tg)
		}
	}

	return
}

func (ySvc *LoadBalancerService) RemoveLBByName(ctx context.Context, name string) error {
	log.Printf("RemoveLBByName start")
	defer log.Printf("RemoveLBByName finished")

	log.Printf("Retrieving LB by name %q", name)
	lb, err := ySvc.GetLbByName(ctx, name)
	if err != nil {
		return err
	}
	if lb == nil {
		log.Printf("LB by Name %q does not exist, skipping deletion\n", name)
		return nil
	}

	lbDeleteRequest := &loadbalancer.DeleteNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lb.Id,
	}

	log.Printf("Deleting LB by ID %q", lb.Id)
	_, _, err = ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		op, err := ySvc.LbSvc.Delete(ctx, lbDeleteRequest)
		logOperation("Removing LB by ID", op, err)
		return op, err
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Printf("LB %q does not exist, skipping\n", name)
		} else {
			return err
		}
	}

	return nil
}

func (ySvc *LoadBalancerService) CreateOrUpdateTG(ctx context.Context, tgName string, targets []*loadbalancer.Target) (string, error) {
	log.Printf("CreateOrUpdateTG start")
	defer log.Printf("CreateOrUpdateTG finished")

	log.Printf("retrieving TargetGroup by name %q", tgName)
	tg, err := ySvc.GetTgByName(ctx, tgName)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Println("TG not found, creating new TG")
		} else {
			return "", err
		}
	}
	if tg == nil {
		tgCreateRequest := &loadbalancer.CreateTargetGroupRequest{
			FolderId: ySvc.cloudCtx.FolderID,
			Name:     tgName,
			RegionId: ySvc.cloudCtx.RegionID,
			Targets:  targets,
		}

		log.Printf("Creating TargetGroup: %+v", *tgCreateRequest)

		result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.TgSvc.Create(ctx, tgCreateRequest)
			logOperation("Creating target group", op, err)
			return op, err
		})
		if err != nil {
			return "", err
		}
		return result.(*loadbalancer.TargetGroup).Id, nil
	}

	dirty := false

	targetsToAdd, targetsToRemove := diffTargetGroupTargets(targets, tg.Targets)
	if len(targetsToAdd) > 0 {
		req := &loadbalancer.AddTargetsRequest{
			TargetGroupId: tg.Id,
			Targets:       targetsToAdd,
		}
		log.Printf("Adding Targets: %+v", *req)

		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.TgSvc.AddTargets(ctx, req)
			logOperation("Adding targets", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}
	if len(targetsToRemove) > 0 {
		req := &loadbalancer.RemoveTargetsRequest{
			TargetGroupId: tg.Id,
			Targets:       targetsToRemove,
		}
		log.Printf("Removing Targets: %+v", *req)

		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			op, err := ySvc.TgSvc.RemoveTargets(ctx, req)
			logOperation("Removing targets", op, err)
			return op, err
		})

		if err != nil {
			return "", err
		}

		dirty = true
	}

	// Ensure that after all manipulations with TargetGroup in the cloud it still exists.
	if dirty {
		log.Printf("Retrieving TargetGroup %q after update", tgName)
		tg, err = ySvc.GetTgByName(ctx, tgName)
		if err != nil {
			return "", err
		}
	}

	return tg.Id, nil
}

func (ySvc *LoadBalancerService) RemoveTGByID(ctx context.Context, tgId string) error {
	log.Printf("RemoveTGByID start")
	defer log.Printf("RemoveTGByID finished")

	tgDeleteRequest := &loadbalancer.DeleteTargetGroupRequest{
		TargetGroupId: tgId,
	}

	log.Printf("Removing TargetGroup: %+v", *tgDeleteRequest)

	_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		op, err := ySvc.TgSvc.Delete(ctx, tgDeleteRequest)
		logOperation("Removing target group", op, err)
		return op, err
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Printf("TG by ID %q does not exist, skipping\n", tgId)
		} else {
			return err
		}
	}

	return nil
}

func (ySvc *LoadBalancerService) GetLbByName(ctx context.Context, name string) (*loadbalancer.NetworkLoadBalancer, error) {
	log.Printf("GetLbByName start")
	defer log.Printf("GetLbByName finished")

	result, err := ySvc.LbSvc.List(ctx, &loadbalancer.ListNetworkLoadBalancersRequest{
		FolderId: ySvc.cloudCtx.FolderID,
		PageSize: 2,
		Filter:   fmt.Sprintf("name = \"%s\"", name),
	})

	if err != nil {
		return nil, err
	}

	if len(result.NetworkLoadBalancers) > 1 {
		return nil, fmt.Errorf("more than 1 LoadBalancers found by the name %q", name)
	}
	if len(result.NetworkLoadBalancers) == 0 {
		return nil, nil
	}

	return result.NetworkLoadBalancers[0], nil
}

func (ySvc *LoadBalancerService) GetTgByName(ctx context.Context, name string) (*loadbalancer.TargetGroup, error) {
	log.Printf("GetTgByName start")
	defer log.Printf("GetTgByName finished")

	result, err := ySvc.TgSvc.List(ctx, &loadbalancer.ListTargetGroupsRequest{
		FolderId: ySvc.cloudCtx.FolderID,
		PageSize: 2,
		Filter:   fmt.Sprintf("name = \"%s\"", name),
	})

	if err != nil {
		return nil, err
	}

	if len(result.TargetGroups) > 1 {
		return nil, fmt.Errorf("more than 1 TargetGroups found by the name %q", name)
	}
	if len(result.TargetGroups) == 0 {
		return nil, nil
	}

	return result.TargetGroups[0], nil
}

func shouldRecreate(oldBalancer *loadbalancer.NetworkLoadBalancer, newBalancerSpec *loadbalancer.CreateNetworkLoadBalancerRequest) bool {
	if newBalancerSpec.Type != oldBalancer.Type {
		log.Println("LB type mismatch, recreating")
		return true
	}

	return false
}

func diffTargetGroupTargets(expectedTargets []*loadbalancer.Target, actualTargets []*loadbalancer.Target) (targetsToAdd []*loadbalancer.Target, targetsToRemove []*loadbalancer.Target) {
	expectedTargetsByUID := make(map[string]*loadbalancer.Target, len(expectedTargets))
	for _, target := range expectedTargets {
		targetUID := fmt.Sprintf("%v:%v", target.SubnetId, target.Address)
		expectedTargetsByUID[targetUID] = target
	}
	actualTargetsByUID := make(map[string]*loadbalancer.Target, len(actualTargets))
	for _, target := range actualTargets {
		targetUID := fmt.Sprintf("%v:%v", target.SubnetId, target.Address)
		actualTargetsByUID[targetUID] = target
	}

	expectedTargetsUIDs := sets.StringKeySet(expectedTargetsByUID)
	actualTargetsUIDs := sets.StringKeySet(actualTargetsByUID)

	for _, targetUID := range expectedTargetsUIDs.Difference(actualTargetsUIDs).List() {
		targetsToAdd = append(targetsToAdd, expectedTargetsByUID[targetUID])
	}
	for _, targetUID := range actualTargetsUIDs.Difference(expectedTargetsUIDs).List() {
		targetsToRemove = append(targetsToRemove, actualTargetsByUID[targetUID])
	}
	return targetsToAdd, targetsToRemove
}

func diffListeners(expectedListeners []*loadbalancer.ListenerSpec, actualListeners []*loadbalancer.Listener) (listenersToAdd []*loadbalancer.ListenerSpec, listenersToRemove []*loadbalancer.Listener) {
	foundSet := make(map[string]bool)

	for _, actual := range actualListeners {
		found := false
		for _, expected := range expectedListeners {
			if nlbListenersAreEqual(actual, expected) {
				// The current listener on the actual
				// nlb is in the set of desired listeners.
				foundSet[expected.Name] = true
				found = true
				break
			}
		}
		if !found {
			listenersToRemove = append(listenersToRemove, actual)
		}
	}

	for _, expected := range expectedListeners {
		if !foundSet[expected.Name] {
			listenersToAdd = append(listenersToAdd, expected)
		}
	}

	return listenersToAdd, listenersToRemove
}

func nlbListenersAreEqual(actual *loadbalancer.Listener, expected *loadbalancer.ListenerSpec) bool {
	if actual.Protocol != expected.Protocol {
		return false
	}
	if actual.Port != expected.Port {
		return false
	}
	if actual.TargetPort != expected.TargetPort {
		return false
	}
	return true
}

func diffAttachedTargetGroups(expectedTGs []*loadbalancer.AttachedTargetGroup, actualTGs []*loadbalancer.AttachedTargetGroup) (tgsToAttach []*loadbalancer.AttachedTargetGroup, tgsToDetach []*loadbalancer.AttachedTargetGroup) {
	foundSet := make(map[string]bool)

	for _, actual := range actualTGs {
		found := false
		for _, expected := range expectedTGs {
			if nlbAttachedTargetGroupsAreEqual(actual, expected) {
				foundSet[expected.TargetGroupId] = true
				found = true
				break
			}
		}
		if !found {
			tgsToDetach = append(tgsToDetach, actual)
		}
	}

	for _, expected := range expectedTGs {
		if !foundSet[expected.TargetGroupId] {
			tgsToAttach = append(tgsToAttach, expected)
		}
	}

	return tgsToAttach, tgsToDetach
}

func nlbAttachedTargetGroupsAreEqual(actual *loadbalancer.AttachedTargetGroup, expected *loadbalancer.AttachedTargetGroup) bool {
	if actual.TargetGroupId != expected.TargetGroupId {
		return false
	}
	if len(actual.HealthChecks) == 0 {
		return false
	}
	actualHealthCheck := actual.HealthChecks[0]
	expectedHealthCheck := expected.HealthChecks[0]
	if actualHealthCheck.Name != expectedHealthCheck.Name {
		return false
	}
	if actualHealthCheck.UnhealthyThreshold != expectedHealthCheck.UnhealthyThreshold {
		return false
	}
	if actualHealthCheck.HealthyThreshold != expectedHealthCheck.HealthyThreshold {
		return false
	}
	actualHealthCheckHttpOptions := actualHealthCheck.GetHttpOptions()
	if actualHealthCheckHttpOptions == nil {
		return false
	}
	expectedHealthCheckHttpOptions := expectedHealthCheck.GetHttpOptions()
	if actualHealthCheckHttpOptions.Port != expectedHealthCheckHttpOptions.Port {
		return false
	}
	if actualHealthCheckHttpOptions.Path != expectedHealthCheckHttpOptions.Path {
		return false
	}
	return true
}
