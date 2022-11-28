package machineautoscaler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// autoscalerCapacityGPU is the annotation key used by infrastructure providers
	// to indicate that a machine will have GPU capacity.
	// ref: https://github.com/openshift/kubernetes-autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/clusterapi_utils.go#L41
	autoscalerCapacityGPU = "machine.openshift.io/GPU"
	// autoscalerGPUAcceleratorLabel is the label name used by the cluster autoscaler
	// to indicate that a node will have a GPU, this is to help the autoscaler wait for GPU drivers to be installed
	// ref: https://github.com/openshift/kubernetes-autoscaler/blob/master/cluster-autoscaler/cloudprovider/clusterapi/clusterapi_provider.go#L40
	autoscalerGPUAcceleratorLabel = "cluster-api/accelerator"
)

var (
	// ErrTargetMissingAnnotations is the error returned when a target is
	// missing the min or max annotations.
	ErrTargetMissingAnnotations = errors.New("missing min or max annotation")

	// ErrTargetAlreadyOwned is the error returned when a target is already
	// marked as owned by another MachineAutoscaler resource.
	ErrTargetAlreadyOwned = errors.New("already owned by another MachineAutoscaler")

	// ErrTargetMissingOwner is the error returned when a target has no owner
	// annotations set.
	ErrTargetMissingOwner = errors.New("missing owner annotation")

	// ErrTargetBadOwner is the error returned when a target's owner annotation
	// is not correctly formatted.
	ErrTargetBadOwner = errors.New("incorrectly formatted owner annotation")
)

// MachineTargetFromObject converts a runtime.Object to a MachineTarget.  Note
// that this does not validate the object is a supported target type.
func MachineTargetFromObject(obj runtime.Object) (*MachineTarget, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	target := &MachineTarget{
		Unstructured: unstructured.Unstructured{
			Object: u,
		},
	}

	return target, nil
}

// MachineTarget represents an unstructured target object for a
// MachineAutoscaler, used to update metadata only.
type MachineTarget struct {
	unstructured.Unstructured
}

// ToUnstructured returns the underlying unstructred object for the target.
func (mt *MachineTarget) ToUnstructured() *unstructured.Unstructured {
	return &mt.Unstructured
}

// NeedsUpdate indicates whether a target needs to be updates to match
// the given min and max values. If there is an error reading the current
// min/max values then this will return true.
// If the machine target has capacity for GPUs but does not contain the
// accelerator label then this function will return true.
func (mt *MachineTarget) NeedsUpdate(min, max int) bool {
	currentMin, currentMax, err := mt.GetLimits()
	if err != nil {
		return true
	}

	minDiff := min != currentMin
	maxDiff := max != currentMax

	// if the machine target has GPU capacity, then it needs a label as well.
	// but, we only update if the label is missing.
	needsGPULabel := (mt.HasGPUAcceleratorLabel() == false) && (mt.HasGPUCapacity() == true)

	return minDiff || maxDiff || needsGPULabel
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
func (mt *MachineTarget) RemoveLimits() bool {
	annotations := []string{
		minSizeAnnotation,
		maxSizeAnnotation,
	}

	return mt.RemoveAnnotations(annotations)
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

// SetOwner sets the target's owner annotation to the given object.  It returns
// a boolean indicating whether the owner annotation changed, and an error,
// which will be ErrTargetAlreadyOwned if the target is already owned.
func (mt *MachineTarget) SetOwner(owner metav1.Object) (bool, error) {
	annotations := mt.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	ownerRef := types.NamespacedName{
		Namespace: owner.GetNamespace(),
		Name:      owner.GetName(),
	}

	if a, ok := annotations[MachineTargetOwnerAnnotation]; ok {
		if a != ownerRef.String() {
			return false, ErrTargetAlreadyOwned
		}

		return false, nil
	}

	annotations[MachineTargetOwnerAnnotation] = ownerRef.String()
	mt.SetAnnotations(annotations)

	return true, nil
}

// RemoveOwner removes the owner annotation from the target.
func (mt *MachineTarget) RemoveOwner() bool {
	annotations := []string{
		MachineTargetOwnerAnnotation,
	}

	return mt.RemoveAnnotations(annotations)
}

// GetOwner returns a types.NamespacedName referencing the target's owner,
// ErrTargetMissingOwner if the target has no owner annotation, or
// ErrTargetBadOwner if the owner annotation is present but malformed.
func (mt *MachineTarget) GetOwner() (types.NamespacedName, error) {
	nn := types.NamespacedName{}
	annotations := mt.GetAnnotations()

	if annotations == nil {
		return nn, ErrTargetMissingOwner
	}

	owner, found := annotations[MachineTargetOwnerAnnotation]
	if !found {
		return nn, ErrTargetMissingOwner
	}

	parts := strings.Split(owner, string(types.Separator))

	if len(parts) != 2 {
		return nn, ErrTargetBadOwner
	}

	nn.Namespace, nn.Name = parts[0], parts[1]

	return nn, nil
}

// RemoveAnnotations removes the annotations with the given keys from the
// target.  It returns a bool indicating whether the annotations were actually
// modified.
func (mt *MachineTarget) RemoveAnnotations(keys []string) bool {
	annotations := mt.GetAnnotations()
	modified := false

	for _, key := range keys {
		if _, found := annotations[key]; found {
			delete(annotations, key)
			modified = true
		}
	}

	mt.SetAnnotations(annotations)

	return modified
}

// Finalize removes autoscaling configuration from the target and returns a bool
// indicating whether the target was actually modified.
func (mt *MachineTarget) Finalize() bool {
	limitsModified := mt.RemoveLimits()
	ownerModified := mt.RemoveOwner()

	return limitsModified || ownerModified
}

// NamespacedName returns a NamespacedName for the target.
func (mt *MachineTarget) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      mt.GetName(),
		Namespace: mt.GetNamespace(),
	}
}

// HasGPUCapacity returns true if the machine target contains the annotation
// which indicates that the target will have GPU capacity, and that the
// value is positive.
func (mt *MachineTarget) HasGPUCapacity() bool {
	annotations := mt.GetAnnotations()
	value, found := annotations[autoscalerCapacityGPU]
	if found {
		quantityGPU := resource.MustParse(value)
		numGPU, converted := quantityGPU.AsInt64()
		if !converted || numGPU < 1 {
			found = false
		}
	}
	return found
}

// HasGPUAcceleratorLabel returns true if the machine target contains the
// label to indicate that the autoscaler should expect nodes from this
// target to contain GPUs.
func (mt *MachineTarget) HasGPUAcceleratorLabel() bool {
	labels := mt.GetLabels()
	// the value of this label is not used, only its presence
	_, foundInMeta := labels[autoscalerGPUAcceleratorLabel]

	labels, _, _ = unstructured.NestedStringMap(mt.Object, "spec", "template", "spec", "metadata", "labels")
	// the value of this label is not used, only its presence
	_, foundInSpecMeta := labels[autoscalerGPUAcceleratorLabel]

	return foundInMeta && foundInSpecMeta
}

// SetGPUAcceleratorLabel adds the label to inform the autoscaler that it
// should expect nodes from this target to contain GPUs.
func (mt *MachineTarget) SetGPUAcceleratorLabel() {
	labels := mt.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	labels[autoscalerGPUAcceleratorLabel] = ""

	mt.SetLabels(labels)

	// because we need new nodes created from this machineset to also have the accerlator
	// label, we add it specifically to the .spec.template.spec.metadata.labels as well.
	// this will ensure that the autoscaler knows these nodes will eventually have a GPU.
	nodelabels, _, _ := unstructured.NestedStringMap(mt.Object, "spec", "template", "spec", "metadata", "labels")

	if nodelabels == nil {
		nodelabels = make(map[string]string)
	}

	nodelabels[autoscalerGPUAcceleratorLabel] = ""

	unstructured.SetNestedStringMap(mt.Object, nodelabels, "spec", "template", "spec", "metadata", "labels")
}
