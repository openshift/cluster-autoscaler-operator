package clusterautoscaler

import (
	"context"
	"sort"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestCreateOrUpdateAutoscalerNetworkPolicies(t *testing.T) {
	r := newFakeReconciler()
	ca := NewClusterAutoscaler()

	expected := r.AutoscalerNetworkPolicies(ca)
	for i := range expected {
		if err := controllerutil.SetControllerReference(ca, &expected[i], r.scheme); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
	var modified []networkingv1.NetworkPolicy
	for _, e := range expected {
		modified = append(modified, *e.DeepCopy())
	}
	for i := range modified {
		if err := controllerutil.SetControllerReference(ca, &modified[i], r.scheme); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		modified[i].Spec = networkingv1.NetworkPolicySpec{}
	}

	var createdResult []controllerutil.OperationResult
	var updatedResult []controllerutil.OperationResult
	for _ = range expected {
		createdResult = append(createdResult, controllerutil.OperationResultCreated)
		updatedResult = append(updatedResult, controllerutil.OperationResultUpdated)
	}
	testCases := []struct {
		current    []networkingv1.NetworkPolicy
		expectedOP []controllerutil.OperationResult
	}{
		{
			current:    []networkingv1.NetworkPolicy{},
			expectedOP: createdResult,
		},
		{
			current:    modified,
			expectedOP: updatedResult,
		},
	}

	for _, tc := range testCases {
		var r *Reconciler
		if tc.current != nil {
			var policyobjs []runtime.Object
			for _, policy := range tc.current {
				policyobjs = append(policyobjs, policy.DeepCopyObject())
			}
			r = newFakeReconciler(policyobjs...)
		} else {
			r = newFakeReconciler()
		}

		op, err := r.createOrUpdateAutoscalerNetworkPolicies(ca)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !equality.Semantic.DeepEqual(op, tc.expectedOP) {
			t.Errorf("expected: %s, got: %s", tc.expectedOP, op)
		}

		fresh := networkingv1.NetworkPolicyList{}
		if err := r.client.List(context.TODO(), &fresh); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		freshlist := fresh.Items
		sort.Slice(freshlist, func(i, j int) bool { return freshlist[i].Name < freshlist[j].Name })
		sort.Slice(expected, func(i, j int) bool { return expected[i].Name < expected[j].Name })

		for i := range freshlist {
			freshlist[i].ResourceVersion = expected[i].ResourceVersion
			// TODO: find a better way to handle this. Added because of https://github.com/kubernetes-sigs/controller-runtime/pull/2633
			freshlist[i].TypeMeta = expected[i].TypeMeta
		}

		if !equality.Semantic.DeepEqual(freshlist, expected) {
			t.Errorf("expected: %v, got: %v", expected, freshlist)
		}
	}
}
