package autoscaler

import (
	"fmt"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
)

func ArgsFromCR(ca *v1alpha1.ClusterAutoscaler) ([]string, error) {
	// TODO(bison): Probably need --write-status-configmap=false
	// because the configmap name is not configurable.
	args := []string{
		"--logtostderr",
		"--cloud-provider=cluster-api",
		fmt.Sprintf("--namespace=%s", ca.Namespace),
		fmt.Sprintf("--scan-interval=%s", ca.Spec.ScanInterval),
	}

	// TODO: ScaleDown should probably be an optional pointer in the spec?
	args = append(args, ScaleDownArgs(&ca.Spec.ScaleDown)...)

	return args, nil
}

func ScaleDownArgs(sd *v1alpha1.ScaleDownConfig) []string {
	if !sd.Enabled {
		return []string{"--scale-down-enabled=false"}
	}

	args := []string{
		"--scale-down-enabled=true",
		fmt.Sprintf("--scale-down-delay-after-add=%s", sd.DelayAfterAdd),
		fmt.Sprintf("--scale-down-delay-after-delete=%s", sd.DelayAfterDelete),
		fmt.Sprintf("--scale-down-delay-after-failure=%s", sd.DelayAfterFailure),
	}

	return args
}
