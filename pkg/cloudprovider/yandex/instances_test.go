package yandex

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	compute "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/deckhouse/yandex-cloud-controller-manager/pkg/yapi"
)

type fakeInstanceServiceClient struct {
	compute.InstanceServiceClient

	instances map[string]*compute.Instance
}

func (f *fakeInstanceServiceClient) Get(_ context.Context, request *compute.GetInstanceRequest, _ ...grpc.CallOption) (*compute.Instance, error) {
	instance, found := f.instances[request.InstanceId]
	if !found {
		return nil, status.Error(codes.NotFound, "instance not found")
	}
	return instance, nil
}

func newTestCloud(instances map[string]*compute.Instance) *Cloud {
	cloudCtx := &yapi.CloudContext{FolderID: "test-folder"}
	return &Cloud{
		config: CloudConfig{FolderID: "test-folder"},
		yandexService: &yapi.YandexCloudAPI{
			ComputeSvc: yapi.NewComputeService(&fakeInstanceServiceClient{instances: instances}, nil, cloudCtx),
		},
	}
}

// Target group synchronization leaves a Node out of the target groups when its Instance is gone,
// and that decision is made by matching cloudprovider.InstanceNotFound. Should this stop being a
// typed error, a single deleted Instance would silently start aborting synchronization for the
// whole cluster again.
func TestGetInstanceByProviderIDReturnsInstanceNotFoundForMissingInstance(t *testing.T) {
	cloud := newTestCloud(map[string]*compute.Instance{
		"instance-1": {Id: "instance-1", Name: "node-1"},
	})

	instance, err := cloud.getInstanceByProviderID(context.Background(), "yandex://instance-1")
	if err != nil {
		t.Fatalf("expected an existing Instance to resolve, got error: %v", err)
	}
	if instance.Id != "instance-1" {
		t.Fatalf("resolved Instance ID = %q, want %q", instance.Id, "instance-1")
	}

	_, err = cloud.getInstanceByProviderID(context.Background(), "yandex://instance-2")
	if !errors.Is(err, cloudprovider.InstanceNotFound) {
		t.Fatalf("expected cloudprovider.InstanceNotFound for a deleted Instance, got %v", err)
	}
}

// A ProviderID that cannot be parsed must not be reported as a missing Instance: skipping such a
// Node would hide a misconfiguration, so it has to surface as an ordinary error instead.
func TestGetInstanceByProviderIDRejectsUnparsableProviderID(t *testing.T) {
	cloud := newTestCloud(nil)

	_, err := cloud.getInstanceByProviderID(context.Background(), "aws:///eu-central-1a/i-1")
	if err == nil {
		t.Fatal("expected an error for a non-Yandex ProviderID")
	}
	if errors.Is(err, cloudprovider.InstanceNotFound) {
		t.Fatalf("an unparsable ProviderID must not be reported as a missing Instance, got %v", err)
	}
}
