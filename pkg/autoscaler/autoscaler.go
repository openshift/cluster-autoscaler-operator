package autoscaler

import (
	"fmt"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
)

func AutoscalerArgs(ca *v1alpha1.ClusterAutoscaler) []string {
	args := []string{
		"--logtostderr",
		"--cloud-provider=cluster-api",
		fmt.Sprintf("--namespace=%s", ca.Namespace),
	}

	if ca.Spec.MaxPodGracePeriod != nil {
		mpgp := fmt.Sprintf("--max-graceful-termination-sec=%d", *ca.Spec.MaxPodGracePeriod)
		args = append(args, mpgp)
	}

	if ca.Spec.PodPriorityThreshold != nil {
		ppt := fmt.Sprintf("--expendable-pods-priority-cutoff=%d", *ca.Spec.PodPriorityThreshold)
		args = append(args, ppt)
	}

	if ca.Spec.ResourceLimits != nil {
		args = append(args, ResourceArgs(ca.Spec.ResourceLimits)...)
	}

	if ca.Spec.ScaleDown != nil {
		args = append(args, ScaleDownArgs(ca.Spec.ScaleDown)...)
	}

	return args
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

func ResourceArgs(rl *v1alpha1.ResourceLimits) []string {
	args := []string{}

	if rl.MaxNodesTotal != nil {
		maxNodes := fmt.Sprintf("--max-nodes-total=%d", *rl.MaxNodesTotal)
		args = append(args, maxNodes)
	}

	if rl.Cores != nil {
		cores := fmt.Sprintf("--cores-total=%s", RangeString(*rl.Cores))
		args = append(args, cores)
	}

	if rl.Memory != nil {
		memory := fmt.Sprintf("--memory-total=%s", RangeString(*rl.Memory))
		args = append(args, memory)
	}

	for _, g := range rl.GPUS {
		gpuRange := RangeString(g.ResourceRange)
		gpu := fmt.Sprintf("--gpu-total=%s:%s", g.Type, gpuRange)
		args = append(args, gpu)
	}

	return args
}

func RangeString(rr v1alpha1.ResourceRange) string {
	return fmt.Sprintf("%d:%d", rr.Min, rr.Max)
}
