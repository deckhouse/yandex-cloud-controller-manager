package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/options"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	_ "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

const (
	// The variables below are samples, please edit the value for your case.

	// sampleCloudProviderName shows an sample of using hard coded parameter for CloudProviderName
	yandexCloudProviderName = "yandexCloudProvider"
	// sampleCloudProviderConfigFile shows an sample of using hard coded parameter for CloudProviderConfigFile
	yandexCloudProviderConfigFile = ""
)

func main() {
	rand.Seed(time.Now().UnixNano())

	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}
	c, err := s.Config([]string{}, app.ControllersDisabledByDefault.List())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	cloud, err := cloudprovider.InitCloudProvider(yandexCloudProviderName, yandexCloudProviderConfigFile)
	if err != nil {
		klog.Fatalf("Cloud provider could not be initialized: %v", err)
	}
	if cloud == nil {
		klog.Fatalf("cloud provider is nil")
	}

	if !cloud.HasClusterID() {
		if c.ComponentConfig.KubeCloudShared.AllowUntaggedCloud {
			klog.Warning("detected a cluster without a ClusterID.  A ClusterID will be required in the future.  Please tag your cluster to avoid any future issues")
		} else {
			klog.Fatalf("no ClusterID found.  A ClusterID is required for the cloud provider to function properly.  This check can be bypassed by setting the allow-untagged-cloud option")
		}
	}

	// Initialize the cloud provider with a reference to the clientBuilder
	cloud.Initialize(c.ClientBuilder, make(chan struct{}))
	// Set the informer on the user cloud object
	if informerUserCloud, ok := cloud.(cloudprovider.InformerUser); ok {
		informerUserCloud.SetInformers(c.SharedInformers)
	}

	controllerInitializers := app.DefaultControllerInitializers(c.Complete(), cloud)
	command := app.NewCloudControllerManagerCommand(s, c, controllerInitializers)
	command.Use = "yandex-cloud-controller-manager"

	pflag.CommandLine.SetNormalizeFunc(flag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	_ = goflag.CommandLine.Parse([]string{})

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
