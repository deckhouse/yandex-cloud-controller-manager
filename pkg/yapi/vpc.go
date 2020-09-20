package yapi

import (
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
)

type VPCService struct {
	cloudCtx *CloudContext

	NetworkSvc    vpc.NetworkServiceClient
	SubnetSvc     vpc.SubnetServiceClient
	RouteTableSvc vpc.RouteTableServiceClient
}

func NewVPCService(nSvc vpc.NetworkServiceClient, sSvc vpc.SubnetServiceClient, rtSvc vpc.RouteTableServiceClient,
	cloudCtx *CloudContext) *VPCService {

	return &VPCService{
		NetworkSvc:    nSvc,
		SubnetSvc:     sSvc,
		RouteTableSvc: rtSvc,

		cloudCtx: cloudCtx,
	}
}
