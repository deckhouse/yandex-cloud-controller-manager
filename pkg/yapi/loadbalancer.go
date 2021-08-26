package yapi

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (ySvc *LoadBalancerService) CreateOrUpdateLB(ctx context.Context, name string, listenerSpec []*loadbalancer.ListenerSpec, attachedTGs []*loadbalancer.AttachedTargetGroup) (string, error) {
	var nlbType = loadbalancer.NetworkLoadBalancer_EXTERNAL
	for _, listener := range listenerSpec {
		if _, ok := listener.Address.(*loadbalancer.ListenerSpec_InternalAddressSpec); ok {
			nlbType = loadbalancer.NetworkLoadBalancer_INTERNAL
			break
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

	log.Printf("Getting LB by name: %q", name)
	lb, err := ySvc.GetLbByName(ctx, name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Println("LB not found, creating new LB")
		} else {
			return "", err
		}
	}

	if lb == nil {
		log.Printf("Creating LoadBalancer: %+v", *lbCreateRequest)

		result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			return ySvc.LbSvc.Create(ctx, lbCreateRequest)
		})
		if err != nil {
			return "", err
		}

		return result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address, nil
	}

	if lb != nil && shouldRecreate(lb, lbCreateRequest) {
		log.Printf("Re-creating LoadBalancer: %+v", *lbCreateRequest)

		_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			return ySvc.LbSvc.Delete(ctx, &loadbalancer.DeleteNetworkLoadBalancerRequest{NetworkLoadBalancerId: lb.Id})
		})
		if err != nil {
			return "", err
		}

		result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
			return ySvc.LbSvc.Create(ctx, lbCreateRequest)
		})
		if err != nil {
			return "", err
		}

		return result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address, nil
	}

	log.Printf("LB %q already exists, attempting an update\n", name)

	lbUpdateRequest := &loadbalancer.UpdateNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lb.Id,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"listeners", "attached_target_groups"},
		},
		ListenerSpecs:        listenerSpec,
		AttachedTargetGroups: attachedTGs,
	}

	log.Printf("Updating LoadBalancer: %+v", *lbUpdateRequest)

	result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		return ySvc.LbSvc.Update(ctx, lbUpdateRequest)
	})
	if err != nil {
		return "", err
	}

	return result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address, nil
}

func (ySvc *LoadBalancerService) GetTGsByClusterName(ctx context.Context, clusterName string) (ret []*loadbalancer.TargetGroup, err error) {
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
		return ySvc.LbSvc.Delete(ctx, lbDeleteRequest)
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
	tgCreateRequest := &loadbalancer.CreateTargetGroupRequest{
		FolderId: ySvc.cloudCtx.FolderID,
		Name:     tgName,
		RegionId: ySvc.cloudCtx.RegionID,
		Targets:  targets,
	}

	log.Printf("Creating TargetGroup: %+v", *tgCreateRequest)

	result, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		return ySvc.TgSvc.Create(ctx, tgCreateRequest)
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			log.Printf("TG by name %q already exists, attempting an update\n", tgName)
		} else if status.Code(err) == codes.InvalidArgument && strings.Contains(status.Convert(err).Message(), "One of the targets already a part of the another target group") {
			log.Printf("TG with the same targets exists already, attempting an update")
		} else {
			return "", err
		}
	} else {
		return result.(*loadbalancer.TargetGroup).Id, nil
	}

	log.Printf("retrieving TargetGroup by name %q", tgName)
	tg, err := ySvc.GetTgByName(ctx, tgName)
	if err != nil {
		return "", err
	}
	if tg == nil {
		return "", fmt.Errorf("TG by name %q does not yet exist", tgName)
	}

	tgUpdateRequest := &loadbalancer.UpdateTargetGroupRequest{
		TargetGroupId: tg.Id,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"targets", "labels"},
		},
		Targets: targets,
	}

	log.Printf("Updating TargetGroup: %+v", *tgUpdateRequest)

	result, _, err = ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		return ySvc.TgSvc.Update(ctx, tgUpdateRequest)
	})
	if err != nil {
		return "", err
	}

	return result.(*loadbalancer.TargetGroup).Id, nil
}

func (ySvc *LoadBalancerService) RemoveTGByID(ctx context.Context, tgId string) error {
	tgDeleteRequest := &loadbalancer.DeleteTargetGroupRequest{
		TargetGroupId: tgId,
	}

	log.Printf("Removing TargetGroup: %+v", *tgDeleteRequest)

	_, _, err := ySvc.cloudCtx.OperationWaiter(ctx, func() (*operation.Operation, error) {
		return ySvc.TgSvc.Delete(ctx, tgDeleteRequest)
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
