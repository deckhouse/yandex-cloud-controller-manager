module github.com/flant/yandex-cloud-controller-manager

go 1.13

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/protobuf v1.3.5
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/onsi/ginkgo v1.11.0 // indirect
	github.com/onsi/gomega v1.8.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.4.1
	github.com/spf13/pflag v1.0.5
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/yandex-cloud/go-genproto v0.0.0-20200514130135-279e4db5b530
	github.com/yandex-cloud/go-sdk v0.0.0-20200514134153-ba2dba3d5f87
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20200323114720-3f67cca34472
	google.golang.org/grpc v1.28.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/cloud-provider v0.17.1
	k8s.io/component-base v0.17.1
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.17.1
)

replace go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200329194405-dd816f0735f8

replace k8s.io/api => k8s.io/api v0.17.1

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.1

replace k8s.io/apimachinery => k8s.io/apimachinery v0.17.1

replace k8s.io/apiserver => k8s.io/apiserver v0.17.1

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.1

replace k8s.io/client-go => k8s.io/client-go v0.17.1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.1

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.1

replace k8s.io/code-generator => k8s.io/code-generator v0.17.1

replace k8s.io/component-base => k8s.io/component-base v0.17.1

replace k8s.io/cri-api => k8s.io/cri-api v0.17.1

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.1

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.1

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.1

replace k8s.io/kubectl => k8s.io/kubectl v0.17.1

replace k8s.io/kubelet => k8s.io/kubelet v0.17.1

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.1

replace k8s.io/metrics => k8s.io/metrics v0.17.1

replace k8s.io/node-api => k8s.io/node-api v0.17.1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.1

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.17.1

replace k8s.io/sample-controller => k8s.io/sample-controller v0.17.1
