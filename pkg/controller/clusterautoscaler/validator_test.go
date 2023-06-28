package clusterautoscaler

import (
	"testing"

	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidate(t *testing.T) {
	client := fakeclient.NewClientBuilder().Build()
	validator := NewValidator("test", client, scheme.Scheme)
	ca := NewClusterAutoscaler()

	testCases := []struct {
		label            string
		expectedOk       bool
		expectedWarnings bool
		caFunc           func() *autoscalingv1.ClusterAutoscaler
	}{
		{
			label:            "ClusterAutoscaler is valid",
			expectedOk:       true,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				return ca.DeepCopy()
			},
		},
		{
			label:            "ClusterAutoscaler name is invalid",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.SetName("invalid-name")
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has negative MaxNodesTotal",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.MaxNodesTotal = pointer.Int32Ptr(-10)
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has negative Cores",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.Cores.Min = -10
				ca.Spec.ResourceLimits.Cores.Max = -10
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has Max Cores lower than Min",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.Cores.Min = 100
				ca.Spec.ResourceLimits.Cores.Max = 10
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has Max GPU lower than Min",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.GPUS[0].Min = 100
				ca.Spec.ResourceLimits.GPUS[0].Max = 10
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has GPU Type with invalid characters",
			expectedOk:       true,
			expectedWarnings: true,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ResourceLimits.GPUS[0].Type = "nvidia.com/gpu"
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has invalid ScaleDown durations",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ScaleDown.DelayAfterAdd = pointer.StringPtr("not-a-duration")
				ca.Spec.ScaleDown.DelayAfterFailure = pointer.StringPtr("not-a-duration")
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has invalid ScaleDown utilizationThreshold",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ScaleDown.UtilizationThreshold = pointer.StringPtr("not-a-float-value")
				return ca
			},
		},
		{
			label:            "ClusterAutoscaler has out of range ScaleDown utilizationThreshold",
			expectedOk:       false,
			expectedWarnings: false,
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := ca.DeepCopy()
				ca.Spec.ScaleDown.UtilizationThreshold = pointer.StringPtr("1.5")
				return ca
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			res := validator.Validate(tc.caFunc())

			if !res.IsValid() && len(res.Errors.Errors()) == 0 {
				t.Error("validation failed, but err is nil")
			}

			if tc.expectedWarnings && len(res.Warnings) == 0 {
				t.Errorf("expected warnings but none were generated")
			}

			if res.IsValid() != tc.expectedOk {
				t.Errorf("invalid resource, got %v, want %v, err: %v", res.IsValid(), tc.expectedOk, res.Errors)
			}
		})
	}
}
