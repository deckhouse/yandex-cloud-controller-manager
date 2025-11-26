package yandex

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/loadbalancer/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	v1 "k8s.io/api/core/v1"
	svchelpers "k8s.io/cloud-provider/service/helpers"
)

const (
	// node annotation to put node to the specific target group
	customTargetGroupNamePrefixAnnotation = "yandex.cpi.flant.com/target-group-name-prefix"
	targetGroupNetworkIdAnnotation        = "yandex.cpi.flant.com/target-group-network-id"
	externalLoadBalancerAnnotation        = "yandex.cpi.flant.com/loadbalancer-external"
	listenerSubnetIdAnnotation            = "yandex.cpi.flant.com/listener-subnet-id"
	listenerAddressIPv4                   = "yandex.cpi.flant.com/listener-address-ipv4"

	// healthcheck options
	healthcheckIntervalSeconds    = "yandex.cpi.flant.com/healthcheck-interval-seconds"
	healthcheckTimeoutSeconds     = "yandex.cpi.flant.com/healthcheck-timeout-seconds"
	healthcheckUnhealthyThreshold = "yandex.cpi.flant.com/healthcheck-unhealthy-threshold"
	healthcheckHealthyThreshold   = "yandex.cpi.flant.com/healthcheck-healthy-threshold"

	nodesHealthCheckPath = "/healthz"
	// NOTE: Please keep the following port in sync with ProxyHealthzPort in pkg/cluster/ports/ports.go
	// ports.ProxyHealthzPort was not used here to avoid dependencies to k8s.io/kubernetes
	// cloud provider which is required as part of the out-of-tree cloud provider efforts.
	// TODO: use a shared constant once ports in pkg/cluster/ports are in a common external repo.
	lbNodesHealthCheckPort = 10256
)

var kubeToYandexServiceProtoMapping = map[v1.Protocol]loadbalancer.Listener_Protocol{
	v1.ProtocolTCP: loadbalancer.Listener_TCP,
	v1.ProtocolUDP: loadbalancer.Listener_UDP,
}

// GetLoadBalancer is an implementation of LoadBalancer.GetLoadBalancer
func (yc *Cloud) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbName := defaultLoadBalancerName(service)

	log.Printf("Retrieving LB by name %q", lbName)
	lb, err := yc.yandexService.LbSvc.GetLbByName(ctx, lbName)
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

// GetLoadBalancerName is an implementation of LoadBalancer.GetLoadBalancerName.
func (yc *Cloud) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return defaultLoadBalancerName(service)
}

// EnsureLoadBalancer is an implementation of LoadBalancer.EnsureLoadBalancer.
func (yc *Cloud) EnsureLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	err := yc.nodeTargetGroupSyncer.SyncTGs(ctx, nodes)
	if err != nil {
		return nil, err
	}

	return yc.ensureLB(ctx, service, nodes)
}

// UpdateLoadBalancer is an implementation of LoadBalancer.UpdateLoadBalancer.
func (yc *Cloud) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, nodes []*v1.Node) error {
	err := yc.nodeTargetGroupSyncer.SyncTGs(ctx, nodes)
	if err != nil {
		return err
	}

	_, err = yc.ensureLB(ctx, service, nodes)
	return err
}

// EnsureLoadBalancerDeleted is an implementation of LoadBalancer.EnsureLoadBalancerDeleted.
func (yc *Cloud) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	lbName := defaultLoadBalancerName(service)

	err := yc.yandexService.LbSvc.RemoveLBByName(ctx, lbName)
	if err != nil {
		return err
	}

	return yc.nodeTargetGroupSyncer.SyncTGs(ctx, []*v1.Node{})
}

func defaultLoadBalancerName(service *v1.Service) string {
	ret := "a" + string(service.UID)
	ret = strings.ReplaceAll(ret, "-", "")
	if len(ret) > 32 {
		ret = ret[:32]
	}
	return ret
}

func (yc *Cloud) ensureLB(ctx context.Context, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// sanity checks
	// current API restrictions
	if len(service.Spec.Ports) > 10 {
		return nil, fmt.Errorf("Yandex.Cloud API does not support more than 10 listener port specifications")
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Nodes provided")
	}

	lbName := defaultLoadBalancerName(service)
	lbParams, err := yc.getLoadBalancerParameters(service)

	if err != nil {
		return nil, fmt.Errorf("error while extracting parameters: %w", err)
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
			TargetPort: int64(svcPort.NodePort),
		}

		if lbParams.internal {
			internalAddressSpec := &loadbalancer.InternalAddressSpec{
				SubnetId: lbParams.listenerSubnetID,
			}

			if len(lbParams.listenerAddressIPv4) > 0 {
				internalAddressSpec.Address = lbParams.listenerAddressIPv4
				internalAddressSpec.IpVersion = loadbalancer.IpVersion_IPV4
			}

			listenerSpec.Address = &loadbalancer.ListenerSpec_InternalAddressSpec{
				InternalAddressSpec: internalAddressSpec,
			}
		} else {
			externalAddressSpec := &loadbalancer.ExternalAddressSpec{}

			if len(lbParams.listenerAddressIPv4) > 0 {
				externalAddressSpec.Address = lbParams.listenerAddressIPv4
				externalAddressSpec.IpVersion = loadbalancer.IpVersion_IPV4
			}

			listenerSpec.Address = &loadbalancer.ListenerSpec_ExternalAddressSpec{
				ExternalAddressSpec: externalAddressSpec,
			}
		}

		listenerSpecs = append(listenerSpecs, listenerSpec)
	}

	hcPath, hcPort := nodesHealthCheckPath, int32(lbNodesHealthCheckPort)
	if svchelpers.RequestsOnlyLocalTraffic(service) {
		// Service requires a special health check, retrieve the OnlyLocal port & path
		hcPath, hcPort = svchelpers.GetServiceHealthCheckPathPort(service)
	}

	healthCheck := &loadbalancer.HealthCheck{
		Name:               "kube-health-check",
		Interval:           &durationpb.Duration{Seconds: 2},
		Timeout:            &durationpb.Duration{Seconds: 1},
		UnhealthyThreshold: 2,
		HealthyThreshold:   2,
		Options: &loadbalancer.HealthCheck_HttpOptions_{
			HttpOptions: &loadbalancer.HealthCheck_HttpOptions{
				Port: int64(hcPort),
				Path: hcPath,
			},
		},
	}

	if lbParams.healthcheckIntervalSeconds > 0 {
		healthCheck.Interval = &durationpb.Duration{Seconds: int64(lbParams.healthcheckIntervalSeconds)}
	}

	if lbParams.healthcheckTimeoutSeconds > 0 {
		healthCheck.Timeout = &durationpb.Duration{Seconds: int64(lbParams.healthcheckTimeoutSeconds)}
	}

	if lbParams.healthcheckUnhealthyThreshold > 0 {
		healthCheck.UnhealthyThreshold = int64(lbParams.healthcheckUnhealthyThreshold)
	}

	if lbParams.healthcheckHealthyThreshold > 0 {
		healthCheck.HealthyThreshold = int64(lbParams.healthcheckHealthyThreshold)
	}

	log.Printf("Health checking on path %q and port %v; interval %v, timeout %v, UnhealthyThreshold %d, HealthyThreshold %d",
		healthCheck.GetHttpOptions().Path,
		healthCheck.GetHttpOptions().Port,
		healthCheck.GetInterval(),
		healthCheck.GetTimeout(),
		healthCheck.GetUnhealthyThreshold(),
		healthCheck.GetHealthyThreshold(),
	)
	healthChecks := []*loadbalancer.HealthCheck{healthCheck}

	tgName := lbParams.targetGroupNamePrefix + yc.config.ClusterName + lbParams.targetGroupNetworkID

	tg, err := yc.yandexService.LbSvc.GetTgByName(ctx, tgName)
	if err != nil {
		return nil, err
	}
	if tg == nil {
		return nil, fmt.Errorf("TG %q does not exist yet", tgName)
	}

	externalIP, err := yc.yandexService.LbSvc.CreateOrUpdateLB(ctx, lbName, listenerSpecs, []*loadbalancer.AttachedTargetGroup{
		{
			TargetGroupId: tg.Id,
			HealthChecks:  healthChecks,
		},
	})
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: externalIP}}}, nil
}

type loadBalancerParameters struct {
	targetGroupNetworkID  string
	targetGroupNamePrefix string
	listenerSubnetID      string
	listenerAddressIPv4   string
	internal              bool

	healthcheckIntervalSeconds    int
	healthcheckTimeoutSeconds     int
	healthcheckUnhealthyThreshold int
	healthcheckHealthyThreshold   int
}

func (yc *Cloud) getLoadBalancerParameters(svc *v1.Service) (lbParams loadBalancerParameters, err error) {
	if value, ok := svc.Annotations[listenerSubnetIdAnnotation]; ok {
		lbParams.internal = true
		lbParams.listenerSubnetID = value
	} else if len(yc.config.lbListenerSubnetID) != 0 {
		lbParams.listenerSubnetID = yc.config.lbListenerSubnetID
		_, isExternal := svc.Annotations[externalLoadBalancerAnnotation]
		lbParams.internal = !isExternal
	}

	if value, ok := svc.Annotations[targetGroupNetworkIdAnnotation]; ok {
		lbParams.targetGroupNetworkID = value
	} else if len(yc.config.lbTgNetworkID) != 0 {
		lbParams.targetGroupNetworkID = yc.config.lbTgNetworkID
	}

	if value, ok := svc.Annotations[listenerAddressIPv4]; ok {
		lbParams.listenerAddressIPv4 = value
	}

	if value, ok := svc.Annotations[customTargetGroupNamePrefixAnnotation]; ok {
		lbParams.targetGroupNamePrefix = value
	}

	if value, ok := svc.Annotations[healthcheckIntervalSeconds]; ok {
		lbParams.healthcheckIntervalSeconds, err = tryAnnotationValueToInt(healthcheckIntervalSeconds, value)
		if err != nil {
			return
		}
	}

	if value, ok := svc.Annotations[healthcheckTimeoutSeconds]; ok {
		lbParams.healthcheckTimeoutSeconds, err = tryAnnotationValueToInt(healthcheckTimeoutSeconds, value)
		if err != nil {
			return
		}
	}

	if value, ok := svc.Annotations[healthcheckHealthyThreshold]; ok {
		lbParams.healthcheckHealthyThreshold, err = tryAnnotationValueToInt(healthcheckHealthyThreshold, value)
		if err != nil {
			return
		}
	}

	if value, ok := svc.Annotations[healthcheckUnhealthyThreshold]; ok {
		lbParams.healthcheckUnhealthyThreshold, err = tryAnnotationValueToInt(healthcheckUnhealthyThreshold, value)
		if err != nil {
			return
		}
	}
	return
}

func tryAnnotationValueToInt(name, value string) (int, error) {
	v, err := strconv.Atoi(value)
	if err != nil {
		return v, fmt.Errorf("can't convert value of annotation %q to int. value: %q, error %w", name, value, err)
	}
	return v, nil
}
