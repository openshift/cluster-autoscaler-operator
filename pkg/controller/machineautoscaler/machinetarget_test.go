package machineautoscaler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestObject is a fake Kubernetes object used as a reference in a
// MachineTarget objects in the test suite.
type TestObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// NewTarget returns a new MachineTarget referencing an TestObject.
func NewTarget() *MachineTarget {
	obj := &TestObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(err)
	}

	return &MachineTarget{
		Unstructured: unstructured.Unstructured{
			Object: u,
		},
	}
}

func TestNeedsUpdate(t *testing.T) {
	target := NewTarget()
	target.SetLimits(4, 6)

	// Different min and max.
	if !target.NeedsUpdate(2, 4) {
		t.Fatal("target should need update")
	}

	// Same min and max.
	if target.NeedsUpdate(4, 6) {
		t.Fatal("target should not need update")
	}

	target.SetLabels(map[string]string{
		minSizeLabel: "not-an-int",
		maxSizeLabel: "not-an-int",
	})

	// Error parsing values.
	if !target.NeedsUpdate(1, 2) {
		t.Fatal("target should need update")
	}
}

func TestSetLimits(t *testing.T) {
	target := NewTarget()
	expectedMin, expectedMax := 2, 4

	target.SetLimits(expectedMin, expectedMax)
	min, max, err := target.GetLimits()
	if err != nil {
		t.Fatalf("error getting limits: %v", err)
	}

	if min != expectedMin || max != expectedMax {
		t.Fatalf("got %d-%d, want %d-%d",
			min, max, expectedMin, expectedMax)
	}
}

func TestGetLimits(t *testing.T) {
	target := NewTarget()

	// No labels.
	_, _, err := target.GetLimits()
	if err != ErrTargetMissingLabels {
		t.Fatal("expected missing labels error")
	}

	// Set bad min label.
	target.SetLabels(map[string]string{
		minSizeLabel: "not-an-int",
		maxSizeLabel: "4",
	})

	_, _, err = target.GetLimits()
	if err == nil {
		t.Fatal("expected bad label error")
	}

	// Set bad max label.
	target.SetLabels(map[string]string{
		minSizeLabel: "2",
		maxSizeLabel: "not-an-int",
	})

	_, _, err = target.GetLimits()
	if err == nil {
		t.Fatal("expected bad label error")
	}

	// Set correct labels.
	expectedMin, expectedMax := 2, 4
	target.SetLimits(expectedMin, expectedMax)

	min, max, err := target.GetLimits()
	if err != nil {
		t.Fatal("error getting limits")
	}

	if min != 2 || max != 4 {
		t.Fatalf("got %d-%d, want %d-%d",
			min, max, expectedMin, expectedMax)
	}
}

func TestRemoveLimits(t *testing.T) {
	target := NewTarget()

	target.SetLimits(2, 4)
	target.RemoveLimits()

	labels := target.GetLabels()

	_, minOK := labels[minSizeLabel]
	_, maxOK := labels[maxSizeLabel]

	if minOK || maxOK {
		t.Fatal("found labels after removal")
	}
}
