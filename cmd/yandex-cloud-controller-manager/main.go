package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"

	_ "github.com/flant/yandex-cloud-controller-manager/pkg/cloudprovider/yandex"
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	command := app.NewCloudControllerManagerCommand()
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
