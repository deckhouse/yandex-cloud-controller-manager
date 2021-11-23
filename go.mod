module github.com/deckhouse/yandex-cloud-controller-manager

go 1.16

require (
	github.com/deckarep/golang-set v1.7.1
	github.com/golang/protobuf v1.5.2
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.26.0
	github.com/yandex-cloud/go-genproto v0.0.0-20200514130135-279e4db5b530
	github.com/yandex-cloud/go-sdk v0.0.0-20200514134153-ba2dba3d5f87
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.38.0
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/cloud-provider v0.22.0
	k8s.io/component-base v0.22.0
	k8s.io/klog/v2 v2.9.0
)
