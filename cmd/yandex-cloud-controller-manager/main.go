/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file should be written by each cloud provider.
// For an minimal working example, please refer to k8s.io/cloud-provider/sample/basic_main.go
// For an advanced example, please refer to k8s.io/cloud-provider/sample/advanced_main.go
// For more details, please refer to k8s.io/kubernetes/cmd/cloud-controller-manager/main.go

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	"k8s.io/cloud-provider/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"

	_ "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
	_ "k8s.io/component-base/metrics/prometheus/clientgo" // load all the prometheus client-go plugins
	_ "k8s.io/component-base/metrics/prometheus/version"  // for version metric registration
	"k8s.io/klog/v2"
)

const (
	yandexCloudProviderName       = "yandex"
	yandexCloudProviderConfigFile = ""
)

func main() {
	rand.Seed(time.Now().UnixNano())

	logs.InitLogs()
	defer logs.FlushLogs()

	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	var controllerInitializers map[string]app.InitFunc
	command := &cobra.Command{
		Use:  "yandex-cloud-controller-manager",
		Long: `yandex-cloud-controller-manager manages YANDEX cloud resources for a Kubernetes cluster.`,
		Run: func(cmd *cobra.Command, args []string) {

			// Default to the yandex provider if not set
			cloudProviderFlag := cmd.Flags().Lookup("cloud-provider")
			if cloudProviderFlag.Value.String() == "" {
				_ = cloudProviderFlag.Value.Set(yandexCloudProviderName)
			}

			cloudProvider := cloudProviderFlag.Value.String()
			if cloudProvider != yandexCloudProviderName {
				klog.Fatalf("unknown cloud provider %s, only %s are supported", cloudProvider, yandexCloudProviderName)
			}

			cliflag.PrintFlags(cmd.Flags())

			c, err := s.Config([]string{}, app.ControllersDisabledByDefault.List())
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			// initialize cloud provider with the cloud provider name and config file provided
			cloud, err := cloudprovider.InitCloudProvider(cloudProvider, yandexCloudProviderConfigFile)
			if err != nil {
				klog.Fatalf("Cloud provider could not be initialized: %v", err)
			}
			if cloud == nil {
				klog.Fatalf("Cloud provider is nil")
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

			controllerInitializers = app.DefaultControllerInitializers(c.Complete(), cloud)

			if err := app.Run(c.Complete(), controllerInitializers, wait.NeverStop); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	fs := command.Flags()
	namedFlagSets := s.Flags(app.KnownControllers(controllerInitializers), app.ControllersDisabledByDefault.List())
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), command.Name())

	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(command.OutOrStdout())
	command.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	command.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
