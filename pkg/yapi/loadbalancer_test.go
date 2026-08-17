package yapi

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	loadbalancer "github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	cloudoperation "github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
	sdkoperation "github.com/yandex-cloud/go-sdk/operation"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type fakeTargetGroupClient struct {
	loadbalancer.TargetGroupServiceClient

	targetGroups map[string]*loadbalancer.TargetGroup
	operations   []string
}

func (f *fakeTargetGroupClient) List(_ context.Context, request *loadbalancer.ListTargetGroupsRequest, _ ...grpc.CallOption) (*loadbalancer.ListTargetGroupsResponse, error) {
	response := &loadbalancer.ListTargetGroupsResponse{}
	for _, targetGroup := range f.targetGroups {
		if request.Filter == "" || request.Filter == fmt.Sprintf("name = %q", targetGroup.Name) {
			response.TargetGroups = append(response.TargetGroups, targetGroup)
		}
	}
	return response, nil
}

func (f *fakeTargetGroupClient) RemoveTargets(_ context.Context, request *loadbalancer.RemoveTargetsRequest, _ ...grpc.CallOption) (*cloudoperation.Operation, error) {
	targetGroup := f.targetGroupByID(request.TargetGroupId)
	remainingTargets := make([]*loadbalancer.Target, 0, len(targetGroup.Targets))
	for _, actualTarget := range targetGroup.Targets {
		shouldRemove := false
		for _, targetToRemove := range request.Targets {
			if actualTarget.SubnetId == targetToRemove.SubnetId && actualTarget.Address == targetToRemove.Address {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			remainingTargets = append(remainingTargets, actualTarget)
		}
	}
	targetGroup.Targets = remainingTargets
	f.operations = append(f.operations, "remove:"+targetGroup.Name)
	return &cloudoperation.Operation{}, nil
}

func (f *fakeTargetGroupClient) AddTargets(_ context.Context, request *loadbalancer.AddTargetsRequest, _ ...grpc.CallOption) (*cloudoperation.Operation, error) {
	targetGroup := f.targetGroupByID(request.TargetGroupId)
	for _, otherTargetGroup := range f.targetGroups {
		if otherTargetGroup.Id == targetGroup.Id {
			continue
		}
		conflictingTargets, _ := diffTargetGroupTargets(request.Targets, otherTargetGroup.Targets)
		if len(conflictingTargets) != len(request.Targets) {
			return nil, fmt.Errorf("target already belongs to %s", otherTargetGroup.Name)
		}
	}

	targetGroup.Targets = append(targetGroup.Targets, request.Targets...)
	f.operations = append(f.operations, "add:"+targetGroup.Name)
	return &cloudoperation.Operation{}, nil
}

func (f *fakeTargetGroupClient) targetGroupByID(id string) *loadbalancer.TargetGroup {
	for _, targetGroup := range f.targetGroups {
		if targetGroup.Id == id {
			return targetGroup
		}
	}
	return nil
}

func TestReconcileTargetGroupsMovesTargetsBetweenGroups(t *testing.T) {
	const (
		clusterName = "cluster"
		defaultTG   = "cluster-network"
		customTG    = "frontendcluster-network"
	)
	target := &loadbalancer.Target{SubnetId: "subnet", Address: "10.0.0.1"}
	fakeClient := &fakeTargetGroupClient{targetGroups: map[string]*loadbalancer.TargetGroup{
		defaultTG: {Id: "default-id", Name: defaultTG},
		customTG:  {Id: "custom-id", Name: customTG, Targets: []*loadbalancer.Target{target}},
	}}
	service := NewLoadBalancerService(nil, fakeClient, &CloudContext{
		FolderID: "folder",
		OperationWaiter: func(_ context.Context, operation func() (*cloudoperation.Operation, error)) (proto.Message, *sdkoperation.Operation, error) {
			_, err := operation()
			return nil, nil, err
		},
	})

	err := service.ReconcileTargetGroups(context.Background(), clusterName, map[string][]*loadbalancer.Target{
		defaultTG: {target},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantOperations := []string{"remove:" + customTG, "add:" + defaultTG}
	if !reflect.DeepEqual(fakeClient.operations, wantOperations) {
		t.Fatalf("unexpected operation order: got %v, want %v", fakeClient.operations, wantOperations)
	}
	if len(fakeClient.targetGroups[customTG].Targets) != 0 {
		t.Fatalf("custom target group still has targets: %v", fakeClient.targetGroups[customTG].Targets)
	}
	if !reflect.DeepEqual(fakeClient.targetGroups[defaultTG].Targets, []*loadbalancer.Target{target}) {
		t.Fatalf("default target group has unexpected targets: %v", fakeClient.targetGroups[defaultTG].Targets)
	}
}
