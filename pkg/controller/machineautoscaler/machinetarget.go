package machineautoscaler

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MachineTarget represents an unstructured target object for a
// MachineAutoscaler, used to update metadata only.
type MachineTarget struct {
	unstructured.Unstructured
}

// NeedsUpdate indicates whether a target needs to be updates to match
// the given min and max values.  An error may be returned if there
// was an error parsing the current values.
func (mt *MachineTarget) NeedsUpdate(min, max int) bool {
	currentMin, currentMax, err := mt.GetLimits()
	if err != nil {
		return true
	}

	minDiff := min != currentMin
	maxDiff := max != currentMax

	return minDiff || maxDiff
}

// SetLimits sets the target's min and max labels.
func (mt *MachineTarget) SetLimits(min, max int) {
	labels := mt.GetLabels()

	labels[minSizeLabel] = strconv.Itoa(min)
	labels[maxSizeLabel] = strconv.Itoa(max)

	mt.SetLabels(labels)
}

// RemoveLimits removes the target's min and max labels.
func (mt *MachineTarget) RemoveLimits() {
	labels := mt.GetLabels()

	delete(labels, minSizeLabel)
	delete(labels, maxSizeLabel)

	mt.SetLabels(labels)
}

// GetLimits returns the target's min and max limits.  An error may be
// returned if the label's contents could not be parsed as integers.
func (mt *MachineTarget) GetLimits() (min, max int, err error) {
	labels := mt.GetLabels()

	minString, minOK := labels[minSizeLabel]
	maxString, maxOK := labels[maxSizeLabel]

	if !minOK || !maxOK {
		return 0, 0, fmt.Errorf("missing min or max label")
	}

	min, err = strconv.Atoi(minString)
	if err != nil {
		return 0, 0, fmt.Errorf("bad min label: %s", minString)
	}

	max, err = strconv.Atoi(maxString)
	if err != nil {
		return 0, 0, fmt.Errorf("bad max label: %s", maxString)
	}

	return min, max, nil
}
