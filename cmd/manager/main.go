package main

import (
	"context"
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

	// Create a cancellable context derived from the signal handler context so
	// that the operator can initiate a graceful shutdown (e.g. when the cluster
	// TLS profile changes and the operator needs to restart to pick it up).
	ctx, cancel := context.WithCancel(signals.SetupSignalHandler())
	defer cancel()

	operator, err := operator.New(ctx, cancel, config)
	if err != nil {
		klog.Fatalf("Failed to create operator: %v", err)
	}

	klog.Info("Starting cluster-autoscaler-operator")
	if err := operator.Start(ctx); err != nil {
		klog.Fatalf("Failed to start operator: %v", err)
	}
}
