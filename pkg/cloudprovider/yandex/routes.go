package yandex

import (
	"context"
	"fmt"

	"github.com/prometheus/common/log"

	v1 "k8s.io/api/core/v1"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"

	"k8s.io/apimachinery/pkg/types"

	"google.golang.org/genproto/protobuf/field_mask"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	cpiRouteLabelsPrefix = "yandex.cpi.flant.com/"
	cpiNodeRoleLabel     = cpiRouteLabelsPrefix + "node-role" // we store Node's name here. The reason for this is lost in time (like tears in rain).
)

func (yc *Cloud) ListRoutes(ctx context.Context, _ string) ([]*cloudprovider.Route, error) {
	req := &vpc.GetRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
	}

	routeTable, err := yc.yandexService.VPCSvc.RouteTableSvc.Get(ctx, req)
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

		// let's verify NextHop relevance
		currentNextHop := staticRoute.NextHop.(*vpc.StaticRoute_NextHopAddress).NextHopAddress
		internalIP, err := yc.getInternalIpByNodeName(nodeName)
		if err != nil {
			log.Infof("Failed to verify NextHop relevance: %s", err)
		} else if currentNextHop != internalIP {
			log.Warnf("Changing %q's NextHop from %s to %s", nodeName, currentNextHop, internalIP)

			filteredStaticRoutes := filterStaticRoutes(routeTable.StaticRoutes, routeFilterTerm{
				termType:        routeFilterAddOrUpdate,
				nodeName:        nodeName,
				destinationCIDR: staticRoute.Destination.(*vpc.StaticRoute_DestinationPrefix).DestinationPrefix,
				nextHop:         internalIP,
			})

			req := &vpc.UpdateRouteTableRequest{
				RouteTableId: yc.config.RouteTableID,
				UpdateMask: &field_mask.FieldMask{
					Paths: []string{"static_routes"},
				},
				StaticRoutes: filteredStaticRoutes,
			}

			_, _, err := yc.yandexService.OperationWaiter(ctx, func() (*operation.Operation, error) { return yc.yandexService.VPCSvc.RouteTableSvc.Update(ctx, req) })
			if err != nil {
				return nil, err
			}
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

	rt, err := yc.yandexService.VPCSvc.RouteTableSvc.Get(ctx, &vpc.GetRouteTableRequest{RouteTableId: yc.config.RouteTableID})
	if err != nil {
		return err
	}

	kubeNodeName := string(route.TargetNode)
	nextHop, err := yc.getInternalIpByNodeName(kubeNodeName)
	if err != nil {
		return err
	}

	newStaticRoutes := filterStaticRoutes(rt.StaticRoutes, routeFilterTerm{
		termType:        routeFilterAddOrUpdate,
		nodeName:        kubeNodeName,
		destinationCIDR: route.DestinationCIDR,
		nextHop:         nextHop,
	})

	req := &vpc.UpdateRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"static_routes"},
		},
		StaticRoutes: newStaticRoutes,
	}

	_, _, err = yc.yandexService.OperationWaiter(ctx, func() (*operation.Operation, error) { return yc.yandexService.VPCSvc.RouteTableSvc.Update(ctx, req) })
	return err
}

func (yc *Cloud) DeleteRoute(ctx context.Context, _ string, route *cloudprovider.Route) error {
	log.Infof("DeleteRoute called with %+v", *route)

	rt, err := yc.yandexService.VPCSvc.RouteTableSvc.Get(ctx, &vpc.GetRouteTableRequest{RouteTableId: yc.config.RouteTableID})
	if err != nil {
		return err
	}

	nodeNameToDelete := string(route.TargetNode)
	newStaticRoutes := filterStaticRoutes(rt.StaticRoutes, routeFilterTerm{
		termType: routeFilterRemove,
		nodeName: nodeNameToDelete,
	})

	req := &vpc.UpdateRouteTableRequest{
		RouteTableId: yc.config.RouteTableID,
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"static_routes"},
		},
		StaticRoutes: newStaticRoutes,
	}

	_, _, err = yc.yandexService.OperationWaiter(ctx, func() (*operation.Operation, error) { return yc.yandexService.VPCSvc.RouteTableSvc.Update(ctx, req) })
	return err
}

func (yc *Cloud) getInternalIpByNodeName(nodeName string) (string, error) {
	kubeNode, err := yc.nodeLister.Get(nodeName)
	if err != nil {
		return "", err
	}

	var targetInternalIP string
	for _, address := range kubeNode.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			targetInternalIP = address.Address
		}
	}
	if len(targetInternalIP) == 0 {
		return "", fmt.Errorf("no InternalIPs found for Node %q", nodeName)
	}

	return targetInternalIP, nil
}

type routeFilterTerm struct {
	termType        routeFilterTermType
	nodeName        string
	destinationCIDR string
	nextHop         string
}

type routeFilterTermType string

const (
	routeFilterAddOrUpdate routeFilterTermType = "AddOrUpdate"
	routeFilterRemove      routeFilterTermType = "Remove"
)

func filterStaticRoutes(staticRoutes []*vpc.StaticRoute, filterTerms ...routeFilterTerm) (ret []*vpc.StaticRoute) {
	var nodeNamesUpdatedSet = make(map[string]struct{})

	for _, existingStaticRoute := range staticRoutes {
		var (
			nodeName string
			ok       bool
		)

		if nodeName, ok = existingStaticRoute.Labels[cpiNodeRoleLabel]; !ok {
			ret = append(ret, existingStaticRoute)
			continue
		}

		var deleteRoute bool
		var routeAppended bool
		for _, filter := range filterTerms {
			if nodeName != filter.nodeName {
				continue
			}

			if filter.termType == routeFilterAddOrUpdate {
				ret = append(ret, &vpc.StaticRoute{
					Destination: &vpc.StaticRoute_DestinationPrefix{DestinationPrefix: filter.destinationCIDR},
					NextHop:     &vpc.StaticRoute_NextHopAddress{NextHopAddress: filter.nextHop},
					Labels:      existingStaticRoute.Labels,
				})

				nodeNamesUpdatedSet[nodeName] = struct{}{}
				routeAppended = true
				break
			}

			if filter.termType == routeFilterRemove {
				log.Infof("Removing %+v StaticRoute from Yandex.Cloud", existingStaticRoute)
				deleteRoute = true
				break
			}
		}

		if !deleteRoute && !routeAppended {
			ret = append(ret, existingStaticRoute)
		}
	}

	// final iteration to add missing routes
	for _, filter := range filterTerms {
		if filter.termType == routeFilterAddOrUpdate {
			if _, updated := nodeNamesUpdatedSet[filter.nodeName]; !updated {
				ret = append(ret, &vpc.StaticRoute{
					Destination: &vpc.StaticRoute_DestinationPrefix{DestinationPrefix: filter.destinationCIDR},
					NextHop:     &vpc.StaticRoute_NextHopAddress{NextHopAddress: filter.nextHop},
					Labels:      map[string]string{cpiNodeRoleLabel: filter.nodeName},
				})
			}
		}
	}

	return
}
