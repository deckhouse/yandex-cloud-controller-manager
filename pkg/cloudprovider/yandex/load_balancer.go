package yandex

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider/service/helpers"
	"strconv"
	"strings"
)

const (
	targetGroupVpcIdAnnotation = "yandex.cpi.flant.com/target-group-vpc-id"
	listenerSubnetIdAnnotation = "yandex.cpi.flant.com/listener-subnet-id"
)

type LoadBalancerService interface {
	LoadBalancerManager
	TargetGroupManager
}

type LoadBalancerManager interface {
	CreateOrUpdateLB(ctx context.Context, id string, listenerSpec []*loadbalancer.ListenerSpec, attachedTGs []*loadbalancer.AttachedTargetGroup) (*v1.LoadBalancerStatus, error)
	GetLbByName(ctx context.Context, name string) (*v1.LoadBalancerStatus, bool, error)
	RemoveLB(ctx context.Context, id string) error
}

type TargetGroupManager interface {
	CreateOrUpdateTG(ctx context.Context, name string, targets []*loadbalancer.Target) (string, error)
	RemoveTG(ctx context.Context, id string) error
}

var kubeToYandexServiceProtoMapping = map[v1.Protocol]loadbalancer.Listener_Protocol{
	v1.ProtocolTCP: loadbalancer.Listener_TCP,
	v1.ProtocolUDP: loadbalancer.Listener_UDP,
}

func (yc *Cloud) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbName := defaultLoadBalancerName(service)

	return yc.api.GetLbByName(ctx, lbName)
}

func (yc *Cloud) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return defaultLoadBalancerName(service)
}

func (yc *Cloud) EnsureLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return yc.ensureLB(ctx, service, nodes)
}

func (yc *Cloud) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) error {
	_, err := yc.ensureLB(ctx, service, nodes)
	return err
}

func (yc *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	lbName := defaultLoadBalancerName(service)

	err := yc.api.RemoveLB(ctx, lbName)
	if err != nil {
		return err
	}

	err = yc.api.RemoveTG(ctx, lbName)

	return err
}

func defaultLoadBalancerName(service *v1.Service) string {
	ret := "a" + string(service.UID)
	ret = strings.Replace(ret, "-", "", -1)
	if len(ret) > 32 {
		ret = ret[:32]
	}
	return ret
}

func (yc *Cloud) ensureLB(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// sanity checks
	// TODO: current API restrictions
	if len(service.Spec.Ports) > 10 {
		return nil, fmt.Errorf("Yandex.Cloud API does not support more than 10 listener port specifications")
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Nodes provided")
	}

	lbName := defaultLoadBalancerName(service)
	lbParams := yc.getLoadBalancerParameters(service)

	var targets []*loadbalancer.Target
	for _, node := range nodes {
		for _, address := range node.Status.Addresses {
			// support only InternalIPs for now
			if address.Type == v1.NodeInternalIP {
				instance, err := yc.api.FindInstanceByFolderAndName(ctx, yc.config.FolderID, MapNodeNameToInstanceName(types.NodeName(node.Name)))
				if err != nil {
					return nil, err
				}

				if len(lbParams.targetGroupVpcID) != 0 {
					newTargets, err := yc.verifyNetworkMembershipOfAllIfaces(ctx, instance, lbParams.targetGroupVpcID)
					if err != nil {
						return nil, errors.WithStack(err)
					}
					targets = append(targets, newTargets...)
				} else {
					targets = append(targets, &loadbalancer.Target{
						SubnetId: instance.NetworkInterfaces[0].SubnetId,
						Address:  address.Address,
					})
				}
			}
		}
	}

	tgId, err := yc.api.CreateOrUpdateTG(ctx, lbName, targets)
	if err != nil {
		return nil, err
	}

	var listenerSpecs []*loadbalancer.ListenerSpec
	for index, svcPort := range service.Spec.Ports {
		listenerName := svcPort.Name
		if len(listenerName) == 0 {
			listenerName = "listener-" + strconv.Itoa(index)
		}

		listenerSpec := &loadbalancer.ListenerSpec{
			Name:       listenerName,
			Port:       int64(svcPort.Port),
			Protocol:   kubeToYandexServiceProtoMapping[svcPort.Protocol],
			Address:    nil,
			TargetPort: int64(svcPort.NodePort),
		}

		if lbParams.internal {
			listenerSpec.Address = &loadbalancer.ListenerSpec_InternalAddressSpec{
				InternalAddressSpec: &loadbalancer.InternalAddressSpec{
					SubnetId: lbParams.listenerSubnetID,
				},
			}
		} else {
			listenerSpec.Address = &loadbalancer.ListenerSpec_ExternalAddressSpec{
				ExternalAddressSpec: &loadbalancer.ExternalAddressSpec{},
			}
		}

		listenerSpecs = append(listenerSpecs, listenerSpec)
	}

	hcPath, hcPort := helpers.GetServiceHealthCheckPathPort(service)
	if len(hcPath) == 0 || hcPort == 0 {
		hcPath = "/"
		hcPort = 80
	}

	// FIXME: Proper fucking healthchecks
	healthChecks := []*loadbalancer.HealthCheck{
		{
			Name:               "kube-health-check",
			UnhealthyThreshold: 2,
			HealthyThreshold:   2,
			Options: &loadbalancer.HealthCheck_HttpOptions_{
				HttpOptions: &loadbalancer.HealthCheck_HttpOptions{
					Port: int64(hcPort),
					Path: hcPath,
				},
			},
		},
	}

	lbStatus, err := yc.api.CreateOrUpdateLB(ctx, lbName, listenerSpecs, []*loadbalancer.AttachedTargetGroup{
		{
			TargetGroupId: tgId,
			HealthChecks:  healthChecks,
		},
	})
	if err != nil {
		return nil, err
	}

	return lbStatus, nil
}

type loadBalancerParameters struct {
	targetGroupVpcID string
	listenerSubnetID string
	internal         bool
}

func (yc *Cloud) getLoadBalancerParameters(svc *v1.Service) (lbParams loadBalancerParameters) {
	if value, ok := svc.ObjectMeta.Annotations[listenerSubnetIdAnnotation]; ok {
		lbParams.internal = true
		lbParams.listenerSubnetID = value
	}

	if value, ok := svc.ObjectMeta.Annotations[targetGroupVpcIdAnnotation]; ok {
		lbParams.targetGroupVpcID = value
	} else if len(yc.config.NetworkID) != 0 {
		lbParams.targetGroupVpcID = yc.config.NetworkID
	}

	return
}

func (yc *Cloud) verifyNetworkMembershipOfAllIfaces(ctx context.Context, instance *compute.Instance, vpcId string) (lbTargets []*loadbalancer.Target, err error) {
	sdk := yc.api.GetSDK()

	// TODO: Implement simple caching mechanism for subnet-VPC membership lookups
	for _, iface := range instance.NetworkInterfaces {
		subnetInfo, err := sdk.VPC().Subnet().Get(ctx, &vpc.GetSubnetRequest{SubnetId: iface.SubnetId})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if subnetInfo.NetworkId == vpcId {
			lbTargets = append(lbTargets, &loadbalancer.Target{
				SubnetId: iface.SubnetId,
				Address:  iface.PrimaryV4Address.Address,
			})
		}
	}

	if len(lbTargets) == 0 {
		return nil, errors.New(fmt.Sprintf("no subnets found to be a member of %q VPC", vpcId))
	}

	return
}
