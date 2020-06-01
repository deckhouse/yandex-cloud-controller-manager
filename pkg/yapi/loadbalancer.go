package yapi

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
)

type YandexLoadBalancerService struct {
	sdk      *ycsdk.SDK
	cloudCtx CloudContext
}

func NewYandexLoadBalancerService(sdk *ycsdk.SDK, cloudCtx CloudContext) *YandexLoadBalancerService {
	return &YandexLoadBalancerService{
		sdk:      sdk,
		cloudCtx: cloudCtx,
	}
}

func (ySvc *YandexLoadBalancerService) CreateOrUpdateLB(ctx context.Context, name string, listenerSpec []*loadbalancer.ListenerSpec, attachedTGs []*loadbalancer.AttachedTargetGroup) (*v1.LoadBalancerStatus, error) {
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
	lb, err := ySvc.getLbByName(ctx, name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Println("LB not found, creating new LB")
		} else {
			return nil, err
		}
	}

	if lb != nil && shouldRecreate(lb, lbCreateRequest) {
		lbDeleteRequest := &loadbalancer.DeleteNetworkLoadBalancerRequest{NetworkLoadBalancerId: lb.Id}
		_, _, err = WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
			return ySvc.sdk.LoadBalancer().NetworkLoadBalancer().Delete(ctx, lbDeleteRequest)
		})
		if err != nil {
			return nil, err
		}
	}

	log.Printf("Creating LoadBalancer: %+v", *lbCreateRequest)

	result, _, err := WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().NetworkLoadBalancer().Create(ctx, lbCreateRequest)
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			log.Printf("LB %q already exists, attempting an update\n", name)
		} else {
			return nil, err
		}
	} else {
		return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{
			{
				// FIXME: only one?
				IP: result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address,
			},
		}}, nil
	}

	lb, err = ySvc.getLbByName(ctx, name)
	if err != nil {
		return nil, err
	}

	lbUpdateRequest := &loadbalancer.UpdateNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lb.Id,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"listeners", "attached_target_groups"},
		},
		ListenerSpecs:        listenerSpec,
		AttachedTargetGroups: attachedTGs,
	}

	log.Printf("Updating LoadBalancer: %+v", *lbUpdateRequest)

	result, _, err = WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().NetworkLoadBalancer().Update(ctx, lbUpdateRequest)
	})
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{
		{
			// FIXME: only one?
			IP: result.(*loadbalancer.NetworkLoadBalancer).Listeners[0].Address,
		},
	}}, nil
}

func (ySvc *YandexLoadBalancerService) GetLbByName(ctx context.Context, name string) (*v1.LoadBalancerStatus, bool, error) {
	log.Printf("Retrieving LB by name %q", name)
	lb, err := ySvc.getLbByName(ctx, name)
	if err != nil {
		return &v1.LoadBalancerStatus{}, false, err
	}

	if lb == nil {
		return &v1.LoadBalancerStatus{}, false, nil
	}

	var lbIngresses []v1.LoadBalancerIngress
	for _, listener := range lb.Listeners {
		lbIngresses = append(lbIngresses, v1.LoadBalancerIngress{
			IP: fmt.Sprintf("%s://%s:%v", strings.ToLower(loadbalancer.Listener_Protocol_name[int32(listener.Protocol)]), listener.Address, listener.Port),
		})
	}

	return &v1.LoadBalancerStatus{Ingress: lbIngresses}, true, nil
}

func (ySvc *YandexLoadBalancerService) RemoveLB(ctx context.Context, name string) error {
	log.Printf("Retrieving LB by name %q", name)
	lb, err := ySvc.getLbByName(ctx, name)
	if err != nil {
		return err
	}
	if lb == nil {
		log.Printf("LB by Name %q does not exists, skipping deletion\n", name)
		return nil
	}

	lbDeleteRequest := &loadbalancer.DeleteNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: lb.Id,
	}

	log.Printf("Deleting LB by ID %q", lb.Id)
	_, _, err = WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().NetworkLoadBalancer().Delete(ctx, lbDeleteRequest)
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

func (ySvc *YandexLoadBalancerService) CreateOrUpdateTG(ctx context.Context, tgName string, targets []*loadbalancer.Target) (string, error) {
	tgCreateRequest := &loadbalancer.CreateTargetGroupRequest{
		FolderId: ySvc.cloudCtx.FolderID,
		Name:     tgName,
		RegionId: ySvc.cloudCtx.RegionID,
		Targets:  targets,
	}

	log.Printf("Creating TargetGroup: %+v", *tgCreateRequest)

	result, _, err := WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().TargetGroup().Create(ctx, tgCreateRequest)
	})
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			log.Printf("TG by name %q already exists, attempting an update\n", tgName)
		} else if status.Code(err) == codes.FailedPrecondition && strings.Contains(status.Convert(err).Message(), "One of the targets already a part of the another target group") {
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

	result, _, err = WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().TargetGroup().Update(ctx, tgUpdateRequest)
	})
	if err != nil {
		return "", err
	}

	return result.(*loadbalancer.TargetGroup).Id, nil
}

// TODO: Think about removing TG after all LBs are gone
func (ySvc *YandexLoadBalancerService) RemoveTG(ctx context.Context, name string) error {
	tg, err := ySvc.GetTgByName(ctx, name)
	if err != nil {
		return err
	}

	tgDeleteRequest := &loadbalancer.DeleteTargetGroupRequest{
		TargetGroupId: tg.Id,
	}

	_, _, err = WaitForResult(ctx, ySvc.sdk, func() (*operation.Operation, error) {
		return ySvc.sdk.LoadBalancer().TargetGroup().Delete(ctx, tgDeleteRequest)
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Printf("TG %q does not exist, skipping\n", name)
		} else {
			return err
		}
	}

	return nil
}

func (ySvc *YandexLoadBalancerService) getLbByName(ctx context.Context, name string) (*loadbalancer.NetworkLoadBalancer, error) {
	result, err := ySvc.sdk.LoadBalancer().NetworkLoadBalancer().List(ctx, &loadbalancer.ListNetworkLoadBalancersRequest{
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

func (ySvc *YandexLoadBalancerService) GetTgByName(ctx context.Context, name string) (*loadbalancer.TargetGroup, error) {
	result, err := ySvc.sdk.LoadBalancer().TargetGroup().List(ctx, &loadbalancer.ListTargetGroupsRequest{
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
