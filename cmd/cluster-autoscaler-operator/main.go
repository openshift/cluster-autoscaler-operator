package main

import (
	"context"
	"runtime"
	"time"

	"github.com/openshift/cluster-autoscaler-operator/pkg/autoscaler"
	sdk "github.com/operator-framework/operator-sdk/pkg/sdk"
	k8sutil "github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const autoscalingv1alpha1 = "autoscaling.openshift.io/v1alpha1"

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func watchClusterAutoscalers(resyncPeriod time.Duration) {
	kind := "ClusterAutoscaler"
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}

	logrus.Infof("Watching %s, %s, %s, %d",
		autoscalingv1alpha1, kind, namespace, resyncPeriod)

	sdk.Watch(autoscalingv1alpha1, kind, namespace, resyncPeriod)
}

func watchMachineAutoscalers(resyncPeriod time.Duration) {

	kind := "MachineAutoscaler"
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}

	logrus.Infof("Watching %s, %s, %s, %d",
		autoscalingv1alpha1, kind, namespace, resyncPeriod)

	sdk.Watch(autoscalingv1alpha1, kind, namespace, resyncPeriod)
}

func main() {
	printVersion()

	sdk.ExposeMetricsPort()
	metrics, err := autoscaler.RegisterOperatorMetrics()
	if err != nil {
		logrus.Errorf("failed to register operator specific metrics: %v", err)
	}

	h := autoscaler.NewHandler(metrics)

	resyncPeriod := time.Duration(5) * time.Second
	watchClusterAutoscalers(resyncPeriod)
	watchMachineAutoscalers(resyncPeriod)

	sdk.Handle(h)
	sdk.Run(context.TODO())
}
