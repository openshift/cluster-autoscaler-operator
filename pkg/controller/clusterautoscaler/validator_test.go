package clusterautoscaler

import (
	"testing"

	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"k8s.io/utils/pointer"
)

func TestValidate(t *testing.T) {
	validator := NewValidator("test")
	ca := NewClusterAutoscaler()

	testCases := []struct {
		label      string
		expectedOk bool
		caFunc     func() *autoscalingv1.ClusterAutoscaler
	}{
		{
			label:      "ClusterAutoscaler is valid",
			expectedOk: true,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				return ca.DeepCopy()
			},
		},
		{
			label:      "ClusterAutoscaler name is invalid",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.SetName("invalid-name")
				return ca
			},
		},
		{
			label:      "ClusterAutoscaler has negative MaxNodesTotal",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.MaxNodesTotal = pointer.Int32Ptr(-10)
				return ca
			},
		},
		{
			label:      "ClusterAutoscaler has negative Cores",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.Cores.Min = -10
				ca.Spec.ResourceLimits.Cores.Max = -10
				return ca
			},
		},
		{
			label:      "ClusterAutoscaler has Max Cores lower than Min",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.Cores.Min = 100
				ca.Spec.ResourceLimits.Cores.Max = 10
				return ca
			},
		},
		{
			label:      "ClusterAutoscaler has Max GPU lower than Min",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.GPUS[0].Min = 100
				ca.Spec.ResourceLimits.GPUS[0].Max = 10
				return ca
			},
		},
		{
			label:      "ClusterAutoscaler has invalid ScaleDown durations",
			expectedOk: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ScaleDown.DelayAfterAdd = pointer.StringPtr("not-a-duration")
				ca.Spec.ScaleDown.DelayAfterFailure = pointer.StringPtr("not-a-duration")
				return ca
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			ok, err := validator.Validate(tc.caFunc())

			if !ok && err == nil {
				t.Error("validation failed, but err is nil")
			}

			if ok != tc.expectedOk {
				t.Errorf("got %v, want %v, err: %v", ok, tc.expectedOk, err)
			}
		})
	}
}
