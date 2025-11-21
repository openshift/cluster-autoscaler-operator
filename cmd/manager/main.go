package main

import (
	"flag"
	"runtime"

	"github.com/openshift/cluster-autoscaler-operator/pkg/operator"
	"github.com/openshift/cluster-autoscaler-operator/pkg/version"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func printVersion() {
	klog.Infof("Go Version: %s", runtime.Version())
	klog.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	klog.Infof("Version: %s", version.String)
}

func main() {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	printVersion()

	// setup the logger for controller-runtime
	ctrl.SetLogger(klogr.New())

	config, err := operator.ConfigFromEnvironment()
	if err != nil {
		klog.Fatalf("Failed to get config from environment: %v", err)
	}

	stopCh := signals.SetupSignalHandler()

	operator, err := operator.New(stopCh, config)
	if err != nil {
		klog.Fatalf("Failed to create operator: %v", err)
	}

	klog.Info("Starting cluster-autoscaler-operator")
	if err := operator.Start(stopCh); err != nil {
		klog.Fatalf("Failed to start operator: %v", err)
	}
}
