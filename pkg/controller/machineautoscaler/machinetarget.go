package machineautoscaler

import (
	"errors"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ErrTargetMissingAnnotations is the error returned when a target is
// missing the min or max annotations.
var ErrTargetMissingAnnotations = errors.New("missing min or max annotation")

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

// SetLimits sets the target's min and max annotations.
func (mt *MachineTarget) SetLimits(min, max int) {
	annotations := mt.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[minSizeAnnotation] = strconv.Itoa(min)
	annotations[maxSizeAnnotation] = strconv.Itoa(max)

	mt.SetAnnotations(annotations)
}

// RemoveLimits removes the target's min and max annotations.
func (mt *MachineTarget) RemoveLimits() {
	annotations := mt.GetAnnotations()

	delete(annotations, minSizeAnnotation)
	delete(annotations, maxSizeAnnotation)

	mt.SetAnnotations(annotations)
}

// GetLimits returns the target's min and max limits.  An error may be
// returned if the annotations's contents could not be parsed as ints.
func (mt *MachineTarget) GetLimits() (min, max int, err error) {
	annotations := mt.GetAnnotations()

	minString, minOK := annotations[minSizeAnnotation]
	maxString, maxOK := annotations[maxSizeAnnotation]

	if !minOK || !maxOK {
		return 0, 0, ErrTargetMissingAnnotations
	}

	min, err = strconv.Atoi(minString)
	if err != nil {
		return 0, 0, fmt.Errorf("bad min annotation: %s", minString)
	}

	max, err = strconv.Atoi(maxString)
	if err != nil {
		return 0, 0, fmt.Errorf("bad max annotation: %s", maxString)
	}

	return min, max, nil
}
