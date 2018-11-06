package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/golang/glog"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller"
	"github.com/openshift/cluster-autoscaler-operator/pkg/operator"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func printVersion() {
	glog.Infof("Go Version: %s", runtime.Version())
	glog.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	glog.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func getConfig() *operator.Config {
	config := operator.NewConfig()

	if caName, ok := os.LookupEnv("CLUSTER_AUTOSCALER_NAME"); ok {
		config.ClusterAutoscalerName = caName
	}

	if caImage, ok := os.LookupEnv("CLUSTER_AUTOSCALER_IMAGE"); ok {
		config.ClusterAutoscalerImage = caImage
	}

	if caNamespace, ok := os.LookupEnv("CLUSTER_AUTOSCALER_NAMESPACE"); ok {
		config.ClusterAutoscalerNamespace = caNamespace
	}

	return config
}

func main() {
	flag.Parse()
	printVersion()

	operatorConfig := getConfig()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		glog.Fatalf("failed to get watch namespace: %v", err)
	}

	// TODO: Expose metrics port after SDK uses controller-runtime's dynamic client
	// sdk.ExposeMetricsPort()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		glog.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace})
	if err != nil {
		glog.Fatal(err)
	}

	glog.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		glog.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr, operatorConfig); err != nil {
		glog.Fatal(err)
	}

	glog.Info("Starting cluster-autoscaler-operator")
	glog.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
