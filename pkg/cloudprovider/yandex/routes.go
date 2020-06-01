package yandex

import (
	"context"
	"fmt"

	"github.com/prometheus/common/log"

	v1 "k8s.io/api/core/v1"

	"github.com/flant/yandex-cloud-controller-manager/pkg/yapi"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"

	"k8s.io/apimachinery/pkg/types"

	"google.golang.org/genproto/protobuf/field_mask"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	cpiRouteLabelsPrefix = "yandex.cpi.flant.com/"
	cpiNodeRoleLabel     = cpiRouteLabelsPrefix + "node-role"
)

func (yc *Cloud) ListRoutes(ctx context.Context, _ string) ([]*cloudprovider.Route, error) {
	req := &vpc.GetRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
	}

	routeTable, err := yc.api.GetSDK().VPC().RouteTable().Get(ctx, req)
	if err != nil {
		return nil, err
	}

	var cpiRoutes []*cloudprovider.Route
	for _, staticRoute := range routeTable.StaticRoutes {
		var (
			nodeName string
			ok       bool
		)

		if nodeName, ok = staticRoute.Labels[cpiNodeRoleLabel]; !ok {
			continue
		}

		cpiRoutes = append(cpiRoutes, &cloudprovider.Route{
			Name:            nodeName,
			TargetNode:      types.NodeName(nodeName),
			DestinationCIDR: staticRoute.Destination.(*vpc.StaticRoute_DestinationPrefix).DestinationPrefix,
		})
	}

	return cpiRoutes, nil
}

func (yc *Cloud) CreateRoute(ctx context.Context, _ string, _ string, route *cloudprovider.Route) error {
	log.Infof("CreateRoute called with %+v", *route)

	rt, err := yc.api.GetSDK().VPC().RouteTable().Get(ctx, &vpc.GetRouteTableRequest{RouteTableId: yc.config.RouteTableID})
	if err != nil {
		return err
	}

	kubeNodeName := string(route.TargetNode)
	kubeNode, err := yc.nodeTargetGroupSyncer.nodeLister.Get(kubeNodeName)
	if err != nil {
		return err
	}

	var targetInternalIP string
	for _, address := range kubeNode.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			targetInternalIP = address.Address
		}
	}
	if len(targetInternalIP) == 0 {
		return fmt.Errorf("no InternalIPs found for Node %q", kubeNodeName)
	}

	newStaticRoutes := append(rt.StaticRoutes, &vpc.StaticRoute{
		Destination: &vpc.StaticRoute_DestinationPrefix{DestinationPrefix: route.DestinationCIDR},
		NextHop:     &vpc.StaticRoute_NextHopAddress{NextHopAddress: targetInternalIP},
		Labels: map[string]string{
			cpiNodeRoleLabel: kubeNodeName,
		},
	})

	req := &vpc.UpdateRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"static_routes"},
		},
		StaticRoutes: newStaticRoutes,
	}

	_, _, err = yapi.WaitForResult(ctx, yc.api.GetSDK(), func() (*operation.Operation, error) { return yc.api.GetSDK().VPC().RouteTable().Update(ctx, req) })
	return err
}

func (yc *Cloud) DeleteRoute(ctx context.Context, _ string, route *cloudprovider.Route) error {
	log.Infof("DeleteRoute called with %+v", *route)

	rt, err := yc.api.GetSDK().VPC().RouteTable().Get(ctx, &vpc.GetRouteTableRequest{RouteTableId: yc.config.RouteTableID})
	if err != nil {
		return err
	}

	nodeNameToDelete := string(route.TargetNode)

	var newStaticRoutes []*vpc.StaticRoute
	for _, existingStaticRoute := range rt.StaticRoutes {
		if nodeName, _ := existingStaticRoute.Labels[cpiNodeRoleLabel]; nodeName == nodeNameToDelete {
			log.Infof("Removing %+v StaticRoute from Yandex.Cloud", existingStaticRoute)
			continue
		}

		newStaticRoutes = append(newStaticRoutes, existingStaticRoute)
	}

	req := &vpc.UpdateRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"static_routes"},
		},
		StaticRoutes: newStaticRoutes,
	}

	_, _, err = yapi.WaitForResult(ctx, yc.api.GetSDK(), func() (*operation.Operation, error) { return yc.api.GetSDK().VPC().RouteTable().Update(ctx, req) })
	return err
}
