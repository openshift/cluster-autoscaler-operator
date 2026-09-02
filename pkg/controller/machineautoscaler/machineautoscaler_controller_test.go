package machineautoscaler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	TestNamespace     = "test"
	machineAPIVersion = "machine.openshift.io/v1beta1"
	clusterAPIVersion = "cluster.x-k8s.io/v1beta2"
	machineSetKind    = "MachineSet"
)

func init() {
	apis.AddToScheme(scheme.Scheme)
}

// Return a MachineTarget targeting a MachineSet with the given name and API version.
// If authoritativeAPI is provided, it is set in status.authoritativeAPI.
func newMachineTarget(name, apiVersion string, authoritativeAPI ...string) *MachineTarget {
	u := &unstructured.Unstructured{}

	u.SetAPIVersion(apiVersion)
	u.SetKind(machineSetKind)
	u.SetName(name)
	u.SetNamespace(TestNamespace)

	if len(authoritativeAPI) > 0 {
		u.Object["spec"] = map[string]interface{}{
			"authoritativeAPI": authoritativeAPI[0],
		}
	}

	target, err := MachineTargetFromObject(u)
	if err != nil {
		panic(err)
	}

	return target
}

// Set the target on the given MachineAutoscaler.
func setTarget(ma *autoscalingv1beta1.MachineAutoscaler, mt *MachineTarget) {
	ma.Spec.ScaleTargetRef = autoscalingv1beta1.CrossVersionObjectReference{
		APIVersion: mt.GetAPIVersion(),
		Kind:       mt.GetKind(),
		Name:       mt.GetName(),
	}
}

// newFakeReconciler returns a new reconcile.Reconciler with a fake client.
func newFakeReconciler(cfg Config, isClusterAPIIntegrationEnabled bool, authoritativeAPIToGVK map[machinev1.MachineAuthority]schema.GroupVersionKind, initObjects ...runtime.Object) *Reconciler {
	fakeClient := fakeclient.
		NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRuntimeObjects(initObjects...).
		WithStatusSubresource(&autoscalingv1beta1.MachineAutoscaler{}).
		Build()
	return &Reconciler{
		client:                         fakeClient,
		scheme:                         scheme.Scheme,
		recorder:                       events.NewFakeRecorder(128),
		isClusterAPIIntegrationEnabled: isClusterAPIIntegrationEnabled,
		authoritativeAPIToGVK:          authoritativeAPIToGVK,
		config:                         cfg,
	}
}

func conditionsWithoutTransitionTime(conditions []metav1.Condition) []metav1.Condition {
	result := make([]metav1.Condition, len(conditions))
	for i, c := range conditions {
		c.LastTransitionTime = metav1.Time{}
		result[i] = c
	}
	return result
}

func TestRemoveSupportedGVK(t *testing.T) {
	var testCases = []struct {
		label  string
		before []schema.GroupVersionKind
		remove []schema.GroupVersionKind
		after  []schema.GroupVersionKind
	}{
		{
			label:  "remove one",
			before: DefaultSupportedTargetGVKs(),
			remove: []schema.GroupVersionKind{
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
			},
			after: []schema.GroupVersionKind{},
		},
		{
			label:  "remove none",
			before: DefaultSupportedTargetGVKs(),
			remove: []schema.GroupVersionKind{},
			after:  DefaultSupportedTargetGVKs(),
		},
		{
			label:  "remove one with clusterAPIGVK",
			before: append(DefaultSupportedTargetGVKs(), clusterAPIGVK),
			remove: []schema.GroupVersionKind{
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
			},
			after: []schema.GroupVersionKind{
				{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineSet"},
			},
		},
		{
			label:  "remove multiple with clusterAPIGVK",
			before: append(DefaultSupportedTargetGVKs(), clusterAPIGVK),
			remove: []schema.GroupVersionKind{
				{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineSet"},
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
			},
			after: []schema.GroupVersionKind{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.label, func(t *testing.T) {
			r := newFakeReconciler(Config{
				Namespace:           TestNamespace,
				SupportedTargetGVKs: tt.before,
			}, false, nil)

			for _, gvk := range tt.remove {
				r.RemoveSupportedGVK(gvk)
			}

			if !reflect.DeepEqual(r.SupportedGVKs(), tt.after) {
				t.Errorf("\ngot:\n%q\nwant:\n%q\n", r.SupportedGVKs(), tt.after)
			}
		})
	}
}

func TestValidateReference(t *testing.T) {
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
				Kind:       machineSetKind,
				APIVersion: "machine.openshift.io/v1beta1",
			},
		},
	}

	r := newFakeReconciler(Config{
		Namespace:           TestNamespace,
		SupportedTargetGVKs: DefaultSupportedTargetGVKs(),
	}, false, nil)

	for _, tt := range validateReferenceTests {
		t.Run(tt.label, func(t *testing.T) {
			valid, err := r.ValidateReference(tt.ref)

			if !valid && err == nil {
				t.Error("reference invalid, but no error returned")
			}

			if valid != tt.expect {
				t.Errorf("got %t, want %t, err: %v", valid, tt.expect, err)
			}
		})
	}
}

func TestHandleTargetChange(t *testing.T) {
	// A target which will not be fetchable via the API.
	missingTarget := newMachineTarget("missing-target", machineAPIVersion, "MachineAPI")

	var testCases = []struct {
		label                          string
		newTarget                      *MachineTarget
		oldTarget                      *MachineTarget
		authoritativeAPIToGVK          map[machinev1.MachineAuthority]schema.GroupVersionKind
		isClusterAPIIntegrationEnabled bool
	}{
		{
			// MachineAutoscaler with no previous target should have the
			// annotations added to the newly set target.
			label:     "no previous target",
			newTarget: newMachineTarget("no-previous-target", machineAPIVersion, "MachineAPI"),
			oldTarget: nil,
		},
		{
			// MachineAutoscaler with missing previous target should have the
			// annotations added to the newly set target.
			label:     "bad previous target",
			newTarget: newMachineTarget("no-previous-target", machineAPIVersion, "MachineAPI"),
			oldTarget: missingTarget,
		},
		{
			// MachineAutoscaler with a previous target, and a new target which
			// is missing, should still remove annotations on previous target.
			label:     "bad new target",
			newTarget: missingTarget,
			oldTarget: newMachineTarget("previous-target", machineAPIVersion, "MachineAPI"),
		},
		{
			// MachineAutoscaler with both previous and new targets found.
			label:     "good targets",
			newTarget: newMachineTarget("new-target", machineAPIVersion, "MachineAPI"),
			oldTarget: newMachineTarget("previous-target", machineAPIVersion, "MachineAPI"),
		},
		{
			// MachineAutoscaler target changed from MachineAPI to ClusterAPI.
			label:     "target changed from MachineAPI to ClusterAPI",
			newTarget: newMachineTarget("new-target", clusterAPIVersion, "ClusterAPI"),
			oldTarget: newMachineTarget("previous-target", machineAPIVersion, "MachineAPI"),
			authoritativeAPIToGVK: map[machinev1.MachineAuthority]schema.GroupVersionKind{
				machinev1.MachineAuthorityMachineAPI: {Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
				machinev1.MachineAuthorityClusterAPI: clusterAPIGVK,
			},
			isClusterAPIIntegrationEnabled: true,
		},
		{
			// MachineAutoscaler with no previous target should have the
			// annotations added to the newly set target with authoritative API enabled.
			label:     "no previous target with authoritativeAPI enabled",
			newTarget: newMachineTarget("no-previous-target", clusterAPIVersion, "ClusterAPI"),
			oldTarget: nil,
			authoritativeAPIToGVK: map[machinev1.MachineAuthority]schema.GroupVersionKind{
				machinev1.MachineAuthorityMachineAPI: {Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
				machinev1.MachineAuthorityClusterAPI: clusterAPIGVK,
			},
			isClusterAPIIntegrationEnabled: true,
		},
	}

	cfg := Config{
		Namespace:           TestNamespace,
		SupportedTargetGVKs: append(DefaultSupportedTargetGVKs(), clusterAPIGVK),
	}

	for _, tt := range testCases {
		t.Run(tt.label, func(t *testing.T) {
			ma := NewMachineAutoscaler()

			maName := types.NamespacedName{
				Namespace: ma.Namespace,
				Name:      ma.Name,
			}

			objects := []runtime.Object{ma}

			// Only add the old target if it's not meant to be missing.
			if tt.oldTarget != nil && tt.oldTarget != missingTarget {
				objects = append(objects, tt.oldTarget)
			}

			// Only add the new target if it's not meant to be missing.
			if tt.newTarget != nil && tt.newTarget != missingTarget {
				objects = append(objects, tt.newTarget)
			}

			r := newFakeReconciler(cfg, tt.isClusterAPIIntegrationEnabled, tt.authoritativeAPIToGVK, objects...)

			// If there's a previous target, first reconcile the
			// MachineAutoscaler with it set.
			if tt.oldTarget != nil {
				setTarget(ma, tt.oldTarget)

				if err := r.client.Update(context.TODO(), ma); err != nil {
					t.Fatalf("Error updating MachineAutoscaler: %v", err)
				}

				r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: maName})

				// Re-fetch the MachineAutoscaler.
				if err := r.client.Get(context.TODO(), maName, ma); err != nil {
					t.Fatalf("Failed to fetch MachineAutoscaler: %v", err)
				}
			}

			// Now set the new target and reconcile again.
			setTarget(ma, tt.newTarget)

			if err := r.client.Update(context.TODO(), ma); err != nil {
				t.Fatalf("Error updating MachineAutoscaler: %v", err)
			}

			r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: maName})

			// Check that the previous target's annotations were removed.
			if tt.oldTarget != nil && tt.oldTarget != missingTarget {
				target := tt.oldTarget.ToUnstructured().DeepCopy()
				targetName := tt.oldTarget.NamespacedName()

				err := r.client.Get(context.TODO(), targetName, target)
				if err != nil {
					t.Fatalf("Failed to fetch target: %v", err)
				}

				annotations := target.GetAnnotations()

				if _, ok := annotations[MachineTargetOwnerAnnotation]; ok {
					t.Error("Previous target has owner annotation")
				}

				if _, ok := annotations[minSizeAnnotation]; ok {
					t.Error("Previous target has min size annotation")
				}

				if _, ok := annotations[maxSizeAnnotation]; ok {
					t.Error("Previous target has max size annotation")
				}
			}

			// Check that the new target has the expected annotations.
			if tt.newTarget != nil && tt.newTarget != missingTarget {
				target := tt.newTarget.ToUnstructured().DeepCopy()
				targetName := tt.newTarget.NamespacedName()

				err := r.client.Get(context.TODO(), targetName, target)
				if err != nil {
					t.Fatalf("Failed to fetch target: %v", err)
				}

				expected := map[string]string{
					MachineTargetOwnerAnnotation: maName.String(),
					minSizeAnnotation:            strconv.Itoa(TestMinReplicas),
					maxSizeAnnotation:            strconv.Itoa(TestMaxReplicas),
				}

				got := target.GetAnnotations()

				if !equality.Semantic.DeepEqual(got, expected) {
					t.Errorf("got %v, want %v", got, expected)
				}
			}
		})
	}
}

func TestValidateAuthoritativeAPI(t *testing.T) {
	authoritativeAPIToGVK := map[machinev1.MachineAuthority]schema.GroupVersionKind{
		machinev1.MachineAuthorityMachineAPI: {Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
		machinev1.MachineAuthorityClusterAPI: clusterAPIGVK,
	}

	testCases := map[string]struct {
		maTargetRef *corev1.ObjectReference
		machineSet  *MachineTarget
		err         error
	}{
		"MachineAPI valid authoritativeAPI": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion, "MachineAPI"),
		},
		"ClusterAPI valid authoritativeAPI": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: clusterAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", clusterAPIVersion, "ClusterAPI"),
		},
		"authoritativeAPI field does not exist": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion),
			err:        ErrAuthoritativeTypeNotExist,
		},
		"authoritativeAPI is empty string": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion, ""),
			err:        ErrAuthoritativeTypeInvalid,
		},
		"authoritativeAPI is Migrating": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion, "Migrating"),
			err:        ErrMigratingAuthoritativeType,
		},
		"authoritativeAPI is unknown value": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion, "test"),
			err:        ErrAuthoritativeTypeInvalid,
		},
		"MachineAPI authoritativeAPI but targetRef is ClusterAPI": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: clusterAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", clusterAPIVersion, "MachineAPI"),
			err:        ErrAuthoritativeTypeUnsupported,
		},
		"ClusterAPI authoritativeAPI but targetRef is MachineAPI": {
			maTargetRef: &corev1.ObjectReference{
				APIVersion: machineAPIVersion,
				Kind:       machineSetKind,
			},
			machineSet: newMachineTarget("test", machineAPIVersion, "ClusterAPI"),
			err:        ErrAuthoritativeTypeUnsupported,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateAuthoritativeAPI(authoritativeAPIToGVK, testCase.maTargetRef, testCase.machineSet)
			if !errors.Is(err, testCase.err) {
				t.Errorf("unexpected error. got %v, want %v", err, testCase.err)
			}
		})
	}
}

func TestCreateConditionFromValidationError(t *testing.T) {
	testCases := map[string]struct {
		err             error
		expectedMessage string
	}{
		"ErrAuthoritativeTypeNotExist": {
			err:             ErrAuthoritativeTypeNotExist,
			expectedMessage: machineAutoscalerAuthoritativeTypeNotExistsMessage,
		},
		"ErrAuthoritativeTypeInvalid": {
			err:             ErrAuthoritativeTypeInvalid,
			expectedMessage: machineAutoscalerInvalidAuthoritativeTypeMessage,
		},
		"ErrAuthoritativeTypeUnsupported": {
			err:             ErrAuthoritativeTypeUnsupported,
			expectedMessage: machineAutoscalerNATargetScaleRefMessage,
		},
		"ErrMigratingAuthoritativeType": {
			err:             ErrMigratingAuthoritativeType,
			expectedMessage: machineAutoscalerMigratingAuthoritativeTypeMessage,
		},
		"wrapped ErrAuthoritativeTypeNotExist": {
			err:             fmt.Errorf("wrap: %w", ErrAuthoritativeTypeNotExist),
			expectedMessage: machineAutoscalerAuthoritativeTypeNotExistsMessage,
		},
		"unknown error falls back to default": {
			err:             errors.New("something unexpected"),
			expectedMessage: machineAutoscalerInvalidAuthoritativeTypeMessage,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := createConditionFromValidationError(tc.err)

			assert.Equal(t, metav1.ConditionFalse, got.Status)
			assert.Equal(t, machineAutoscalerReadyType, got.Type)
			assert.Equal(t, machineAutoscalerNATargetScaleRefReason, got.Reason)
			assert.Equal(t, tc.expectedMessage, got.Message)
		})
	}
}

func TestUpdateMachineAutoscalerConditions(t *testing.T) {
	testCases := map[string]struct {
		initialConditions  []metav1.Condition
		newCondition       metav1.Condition
		expectedConditions []metav1.Condition
	}{
		"add condition to empty list": {
			initialConditions: nil,
			newCondition: metav1.Condition{
				Type:    machineAutoscalerReadyType,
				Status:  metav1.ConditionTrue,
				Reason:  machineAutoscalerReadyReason,
				Message: machineAutoscalerReadyMessage,
			},
			expectedConditions: []metav1.Condition{
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionTrue,
					Reason:  machineAutoscalerReadyReason,
					Message: machineAutoscalerReadyMessage,
				},
			},
		},
		"add condition with different reason appends": {
			initialConditions: []metav1.Condition{
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionTrue,
					Reason:  machineAutoscalerReadyReason,
					Message: machineAutoscalerReadyMessage,
				},
			},
			newCondition: metav1.Condition{
				Type:    machineAutoscalerReadyType,
				Status:  metav1.ConditionFalse,
				Reason:  machineAutoscalerNATargetScaleRefReason,
				Message: machineAutoscalerNATargetScaleRefMessage,
			},
			expectedConditions: []metav1.Condition{
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionTrue,
					Reason:  machineAutoscalerReadyReason,
					Message: machineAutoscalerReadyMessage,
				},
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionFalse,
					Reason:  machineAutoscalerNATargetScaleRefReason,
					Message: machineAutoscalerNATargetScaleRefMessage,
				},
			},
		},
		"update existing condition with same reason": {
			initialConditions: []metav1.Condition{
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionFalse,
					Reason:  machineAutoscalerReadyReason,
					Message: "old message",
				},
			},
			newCondition: metav1.Condition{
				Type:    machineAutoscalerReadyType,
				Status:  metav1.ConditionTrue,
				Reason:  machineAutoscalerReadyReason,
				Message: machineAutoscalerReadyMessage,
			},
			expectedConditions: []metav1.Condition{
				{
					Type:    machineAutoscalerReadyType,
					Status:  metav1.ConditionTrue,
					Reason:  machineAutoscalerReadyReason,
					Message: machineAutoscalerReadyMessage,
				},
			},
		},
	}

	cfg := Config{
		Namespace:           TestNamespace,
		SupportedTargetGVKs: append(DefaultSupportedTargetGVKs(), clusterAPIGVK),
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ma := NewMachineAutoscaler()
			r := newFakeReconciler(cfg, false, nil, ma)

			ma.Status.Conditions = tc.initialConditions
			if err := r.client.Status().Update(context.TODO(), ma); err != nil {
				t.Fatalf("Error setting initial conditions: %v", err)
			}

			if err := r.updateMachineAutoscalerConditions(ma, tc.newCondition); err != nil {
				t.Fatalf("Error updating conditions: %v", err)
			}

			updatedMA := &autoscalingv1beta1.MachineAutoscaler{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: ma.Namespace, Name: ma.Name}, updatedMA); err != nil {
				t.Fatalf("Error fetching updated MachineAutoscaler: %v", err)
			}

			assert.Len(t, updatedMA.Status.Conditions, len(tc.expectedConditions))
			assert.ElementsMatch(t, tc.expectedConditions, conditionsWithoutTransitionTime(updatedMA.Status.Conditions))
		})
	}
}
