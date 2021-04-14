package machineautoscaler

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const TestNamespace = "test"

func init() {
	apis.AddToScheme(scheme.Scheme)
}

// Return a MachineTarget targeting a MachineSet with the given name.
func newMachineTarget(name string) *MachineTarget {
	u := &unstructured.Unstructured{}

	u.SetAPIVersion("machine.openshift.io/v1beta1")
	u.SetKind("MachineSet")
	u.SetName(name)
	u.SetNamespace(TestNamespace)

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
func newFakeReconciler(cfg Config, initObjects ...runtime.Object) *Reconciler {
	fakeClient := fakeclient.NewFakeClient(initObjects...)
	return &Reconciler{
		client:   fakeClient,
		scheme:   scheme.Scheme,
		recorder: record.NewFakeRecorder(128),
		config:   cfg,
	}
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
				{Group: "cluster.k8s.io", Version: "v1beta1", Kind: "MachineDeployment"},
			},
			after: []schema.GroupVersionKind{
				{Group: "cluster.k8s.io", Version: "v1beta1", Kind: "MachineSet"},
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineDeployment"},
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
			},
		},
		{
			label:  "remove multiple",
			before: DefaultSupportedTargetGVKs(),
			remove: []schema.GroupVersionKind{
				{Group: "cluster.k8s.io", Version: "v1beta1", Kind: "MachineDeployment"},
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineSet"},
			},
			after: []schema.GroupVersionKind{
				{Group: "cluster.k8s.io", Version: "v1beta1", Kind: "MachineSet"},
				{Group: "machine.openshift.io", Version: "v1beta1", Kind: "MachineDeployment"},
			},
		},
		{
			label:  "remove none",
			before: DefaultSupportedTargetGVKs(),
			remove: []schema.GroupVersionKind{},
			after:  DefaultSupportedTargetGVKs(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.label, func(t *testing.T) {
			r := newFakeReconciler(Config{
				Namespace:           TestNamespace,
				SupportedTargetGVKs: tt.before,
			})

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
				Kind:       "MachineSet",
				APIVersion: "cluster.k8s.io/v1beta1",
			},
		},
	}

	r := newFakeReconciler(Config{
		Namespace:           TestNamespace,
		SupportedTargetGVKs: DefaultSupportedTargetGVKs(),
	})

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
	missingTarget := newMachineTarget("missing-target")

	var testCases = []struct {
		label     string
		newTarget *MachineTarget
		oldTarget *MachineTarget
	}{
		{
			// MachineAutoscaler with no previous target should have the
			// annotations added to the newly set target.
			label:     "no previous target",
			newTarget: newMachineTarget("no-previous-target"),
			oldTarget: nil,
		},
		{
			// MachineAutoscaler with missing previous target should have the
			// annotations added to the newly set target.
			label:     "bad previous target",
			newTarget: newMachineTarget("no-previous-target"),
			oldTarget: missingTarget,
		},
		{
			// MachineAutoscaler with a previous target, and a new target which
			// is missing, should still remove annotations on previous target.
			label:     "bad new target",
			newTarget: missingTarget,
			oldTarget: newMachineTarget("previous-target"),
		},
		{
			// MachineAutoscaler with both previous and new targets found.
			label:     "good targets",
			newTarget: newMachineTarget("new-target"),
			oldTarget: newMachineTarget("previous-target"),
		},
	}

	cfg := Config{
		Namespace:           TestNamespace,
		SupportedTargetGVKs: DefaultSupportedTargetGVKs(),
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

			r := newFakeReconciler(cfg, objects...)

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
