package machineautoscaler

import (
	"testing"

	autoscalingv1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewMachineAutoscaler() *autoscalingv1beta1.MachineAutoscaler {
	return &autoscalingv1beta1.MachineAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MachineAutoscaler",
			APIVersion: "autoscaling.openshift.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: autoscalingv1beta1.MachineAutoscalerSpec{
			MinReplicas: 2,
			MaxReplicas: 8,
			ScaleTargetRef: autoscalingv1beta1.CrossVersionObjectReference{
				APIVersion: "machine.openshift.io/v1beta1",
				Kind:       "MachineSet",
				Name:       "test",
			},
		},
	}
}

func TestValidate(t *testing.T) {
	validator := NewValidator()
	ma := NewMachineAutoscaler()

	testCases := []struct {
		label      string
		expectedOk bool
		maFunc     func() *autoscalingv1beta1.MachineAutoscaler
	}{
		{
			label:      "MachineAutoscaler is valid",
			expectedOk: true,
			maFunc: func() *autoscalingv1beta1.MachineAutoscaler {
				return ma.DeepCopy()
			},
		},
		{
			label:      "MachineAutoscaler has negative MinReplicas",
			expectedOk: false,
			maFunc: func() *autoscalingv1beta1.MachineAutoscaler {
				ma := ma.DeepCopy()
				ma.Spec.MinReplicas = -10
				return ma
			},
		},
		{
			label:      "MachineAutoscaler has negative MaxReplicas",
			expectedOk: false,
			maFunc: func() *autoscalingv1beta1.MachineAutoscaler {
				ma := ma.DeepCopy()
				ma.Spec.MaxReplicas = -10
				return ma
			},
		},
		{
			label:      "MachineAutoscaler has MaxReplicas lower than MinReplicas",
			expectedOk: false,
			maFunc: func() *autoscalingv1beta1.MachineAutoscaler {
				ma := ma.DeepCopy()
				ma.Spec.MinReplicas = 8
				ma.Spec.MaxReplicas = 2
				return ma
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			ok, err := validator.Validate(tc.maFunc())

			if !ok && err == nil {
				t.Error("validation failed, but err is nil")
			}

			if ok != tc.expectedOk {
				t.Errorf("got %v, want %v, err: %v", ok, tc.expectedOk, err)
			}
		})
	}
}
