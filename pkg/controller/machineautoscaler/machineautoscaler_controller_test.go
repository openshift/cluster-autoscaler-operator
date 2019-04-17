package machineautoscaler

import (
	"reflect"
	"testing"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const TestNamespace = "test-namespace"

func init() {
	apis.AddToScheme(scheme.Scheme)
}

// newFakeReconciler returns a new reconcile.Reconciler with a fake client.
func newFakeReconciler(cfg *Config, initObjects ...runtime.Object) *Reconciler {
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
			r := newFakeReconciler(&Config{
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

	r := newFakeReconciler(&Config{
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
