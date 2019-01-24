package machineautoscaler

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

var validateReferenceTests = []struct {
	label  string
	expect bool
	ref    *corev1.ObjectReference
}{
	{
		label:  "nil reference",
		expect: false,
		ref:    nil,
	},
	{
		label:  "no name",
		expect: false,
		ref:    &corev1.ObjectReference{},
	},
	{
		label:  "unsupported gvk",
		expect: false,
		ref: &corev1.ObjectReference{
			Name:       "test",
			Kind:       "bad",
			APIVersion: "bad",
		},
	},
	{
		label:  "valid reference",
		expect: true,
		ref: &corev1.ObjectReference{
			Name:       "test",
			Kind:       "MachineSet",
			APIVersion: "cluster.k8s.io/v1alpha1",
		},
	},
}

func TestValidateReference(t *testing.T) {
	for _, tt := range validateReferenceTests {
		t.Run(tt.label, func(t *testing.T) {
			valid, err := ValidateReference(tt.ref)

			if !valid && err == nil {
				t.Error("reference invalid, but no error returned")
			}

			if valid != tt.expect {
				t.Errorf("got %t, want %t, err: %v", valid, tt.expect, err)
			}
		})
	}
}
