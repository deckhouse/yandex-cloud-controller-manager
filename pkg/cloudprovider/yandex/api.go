package yandex

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	ycsdk "github.com/yandex-cloud/go-sdk"
	ycsdkoperation "github.com/yandex-cloud/go-sdk/operation"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
)

const (
	apiDefaultPageSize = 100
)

// CloudAPI is an abstraction over Yandex.Cloud SDK, to allow mocking/unit testing
type CloudAPI interface {
	// FindInstanceByFolderAndName searches for Instance with the specified folderID and instanceName.
	// If nothing found - no error must be returned.
	FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error)

	LoadBalancerService

	RawAPI
}

type RawAPI interface {
	GetSDK() *ycsdk.SDK
}

// YandexCloudAPI is an implementation of CloudAPI
type YandexCloudAPI struct {
	sdk *ycsdk.SDK

	folderID string
	regionID string
}

// NewCloudAPI creates new instance of CloudAPI object
func NewYandexCloudAPI(config *CloudConfig) (CloudAPI, error) {
	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: config.Credentials,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex.Cloud SDK: %s", err)
	}

	return &YandexCloudAPI{
		sdk:      sdk,
		folderID: config.FolderID,
		regionID: "ru-central1",
	}, nil
}

func (api *YandexCloudAPI) GetSDK() *ycsdk.SDK {
	return api.sdk
}

func (api *YandexCloudAPI) CreateOrUpdateLB(ctx context.Context, name string, listenerSpec []*loadbalancer.ListenerSpec, attachedTGs []*loadbalancer.AttachedTargetGroup) (*v1.LoadBalancerStatus, error) {
	var nlbType = loadbalancer.NetworkLoadBalancer_EXTERNAL
	for _, listener := range listenerSpec {
		if _, ok := listener.Address.(*loadbalancer.ListenerSpec_InternalAddressSpec); ok {
			nlbType = loadbalancer.NetworkLoadBalancer_INTERNAL
			break
		}
	}

	lbCreateRequest := &loadbalancer.CreateNetworkLoadBalancerRequest{
		FolderId:             api.folderID,
		Name:                 name,
		RegionId:             api.regionID,
		Type:                 nlbType,
		ListenerSpecs:        listenerSpec,
		AttachedTargetGroups: attachedTGs,
	}

	log.Printf("Getting LB by name: %q", name)
	lb, err := api.getLbByName(ctx, name)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Println("LB not found, creating new LB")
		} else {
			return nil, err
		}
	}

	if lb != nil && shouldRecreate(lb, lbCreateRequest) {
		lbDeleteRequest := &loadbalancer.DeleteNetworkLoadBalancerRequest{NetworkLoadBalancerId: lb.Id}
		_, _, err = api.waitForResult(ctx, func() (*operation.Operation, error) {
			return api.sdk.LoadBalancer().NetworkLoadBalancer().Delete(ctx, lbDeleteRequest)
		})
		if err != nil {
			return nil, err
		}
	}

	log.Printf("Creating LoadBalancer: %+v", *lbCreateRequest)

	result, _, err := api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().NetworkLoadBalancer().Create(ctx, lbCreateRequest)
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

	lb, err = api.getLbByName(ctx, name)
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

	result, _, err = api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().NetworkLoadBalancer().Update(ctx, lbUpdateRequest)
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

func (api *YandexCloudAPI) GetLbByName(ctx context.Context, name string) (*v1.LoadBalancerStatus, bool, error) {
	log.Printf("Retrieving LB by name %q", name)
	lb, err := api.getLbByName(ctx, name)
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

func (api *YandexCloudAPI) RemoveLB(ctx context.Context, name string) error {
	log.Printf("Retrieving LB by name %q", name)
	lb, err := api.getLbByName(ctx, name)
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

	fmt.Printf("Deleting LB by ID %q", lb.Id)
	_, _, err = api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().NetworkLoadBalancer().Delete(ctx, lbDeleteRequest)
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

func (api *YandexCloudAPI) CreateOrUpdateTG(ctx context.Context, tgName string, targets []*loadbalancer.Target) (string, error) {
	tgCreateRequest := &loadbalancer.CreateTargetGroupRequest{
		FolderId: api.folderID,
		Name:     tgName,
		RegionId: api.regionID,
		Targets:  targets,
	}

	log.Printf("Creating TargetGroup: %+v", *tgCreateRequest)

	result, _, err := api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().TargetGroup().Create(ctx, tgCreateRequest)
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

	fmt.Printf("retrieving TargetGroup by name %q", tgName)
	tg, err := api.GetTgByName(ctx, tgName)
	if err != nil {
		return "", err
	}

	tgUpdateRequest := &loadbalancer.UpdateTargetGroupRequest{
		TargetGroupId: tg.Id,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"targets", "labels"},
		},
		Targets: targets,
	}

	log.Printf("Updating TargetGroup: %+v", *tgUpdateRequest)

	result, _, err = api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().TargetGroup().Update(ctx, tgUpdateRequest)
	})
	if err != nil {
		return "", err
	}

	return result.(*loadbalancer.TargetGroup).Id, nil
}

// TODO: Think about removing TG after all LBs are gone
func (api *YandexCloudAPI) RemoveTG(ctx context.Context, name string) error {
	// trying to get TG with the same name as LB
	tg, err := api.GetTgByName(ctx, name)
	if err != nil {
		return err
	}

	tgDeleteRequest := &loadbalancer.DeleteTargetGroupRequest{
		TargetGroupId: tg.Id,
	}

	_, _, err = api.waitForResult(ctx, func() (*operation.Operation, error) {
		return api.sdk.LoadBalancer().TargetGroup().Delete(ctx, tgDeleteRequest)
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

// FindInstanceByFolderAndName searches for Instance with the specified folderID and instanceName.
func (api *YandexCloudAPI) FindInstanceByFolderAndName(ctx context.Context, folderID string, instanceName string) (*compute.Instance, error) {
	result, err := api.sdk.Compute().Instance().List(ctx, &compute.ListInstancesRequest{
		FolderId: folderID,
		Filter:   fmt.Sprintf(`%s = "%s"`, "name", instanceName),
		PageSize: apiDefaultPageSize,
	})
	if err != nil {
		return nil, err
	}

	if result.Instances == nil || len(result.Instances) == 0 {
		return nil, nil
	}

	if len(result.Instances) > 1 {
		return nil, fmt.Errorf("multiple instances found: folderID=%s, instanceName=%s", folderID, instanceName)
	}

	return result.Instances[0], nil
}

func (api *YandexCloudAPI) getLbByName(ctx context.Context, name string) (*loadbalancer.NetworkLoadBalancer, error) {
	result, err := api.sdk.LoadBalancer().NetworkLoadBalancer().List(ctx, &loadbalancer.ListNetworkLoadBalancersRequest{
		FolderId: api.folderID,
		PageSize: 2,
		Filter:   "name=" + strconv.Quote(name),
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

func (api *YandexCloudAPI) GetTgByName(ctx context.Context, name string) (*loadbalancer.TargetGroup, error) {
	result, err := api.sdk.LoadBalancer().TargetGroup().List(ctx, &loadbalancer.ListTargetGroupsRequest{
		FolderId: api.folderID,
		PageSize: 2,
		Filter:   "name=" + strconv.Quote(name),
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

func (api *YandexCloudAPI) waitForResult(ctx context.Context, origFunc func() (*operation.Operation, error)) (proto.Message, *ycsdkoperation.Operation, error) {
	op, err := api.sdk.WrapOperation(origFunc())
	if err != nil {
		return nil, nil, err
	}

	err = op.Wait(ctx)
	if err != nil {
		return nil, op, err
	}

	resp, err := op.Response()
	if err != nil {
		return nil, op, err
	}

	return resp, op, nil
}

func shouldRecreate(oldBalancer *loadbalancer.NetworkLoadBalancer, newBalancerSpec *loadbalancer.CreateNetworkLoadBalancerRequest) bool {
	if newBalancerSpec.Type != oldBalancer.Type {
		log.Println("LB type mismatch, recreating")
		return true
	}

	return false
}
