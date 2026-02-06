package clusterautoscaler

import (
	"context"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestCreateOrUpdateAutoscalerService(t *testing.T) {
	r := newFakeReconciler()
	ca := NewClusterAutoscaler()

	expected := r.AutoscalerService(ca)
	if err := controllerutil.SetControllerReference(ca, expected, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	modified := expected.DeepCopy()
	if err := controllerutil.SetControllerReference(ca, modified, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	modified.Spec.Type = corev1.ServiceTypeNodePort

	testCases := []struct {
		current    *corev1.Service
		expectedOP controllerutil.OperationResult
	}{
		{
			current:    nil,
			expectedOP: controllerutil.OperationResultCreated,
		},
		{
			current:    modified,
			expectedOP: controllerutil.OperationResultUpdated,
		},
	}

	for _, tc := range testCases {
		var r *Reconciler
		if tc.current != nil {
			r = newFakeReconciler(tc.current)
		} else {
			r = newFakeReconciler()
		}

		op, err := r.createOrUpdateAutoscalerService(ca)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if op != tc.expectedOP {
			t.Errorf("expected: %s, got: %s", tc.expectedOP, op)
		}

		fresh := &corev1.Service{}
		if err := r.client.Get(context.TODO(), r.AutoscalerName(ca), fresh); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected.ResourceVersion = fresh.ResourceVersion
		// TODO: find a better way to handle this. Added because of https://github.com/kubernetes-sigs/controller-runtime/pull/2633
		fresh.TypeMeta = expected.TypeMeta
		if !equality.Semantic.DeepEqual(fresh, expected) {
			t.Errorf("expected: %v, got: %v", expected, fresh)
		}
	}
}

func TestCreateOrUpdateAutoscalerServiceMonitopr(t *testing.T) {
	r := newFakeReconciler()
	ca := NewClusterAutoscaler()

	expected := r.AutoscalerServiceMonitor(ca)
	if err := controllerutil.SetControllerReference(ca, expected, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	modified := expected.DeepCopy()
	if err := controllerutil.SetControllerReference(ca, modified, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	modified.Spec = monitoringv1.ServiceMonitorSpec{}

	testCases := []struct {
		current    *monitoringv1.ServiceMonitor
		expectedOP controllerutil.OperationResult
	}{
		{
			current:    nil,
			expectedOP: controllerutil.OperationResultCreated,
		},
		{
			current:    modified,
			expectedOP: controllerutil.OperationResultUpdated,
		},
	}

	for _, tc := range testCases {
		var r *Reconciler
		if tc.current != nil {
			r = newFakeReconciler(tc.current)
		} else {
			r = newFakeReconciler()
		}

		op, err := r.createOrUpdateAutoscalerServiceMonitor(ca)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if op != tc.expectedOP {
			t.Errorf("expected: %s, got: %s", tc.expectedOP, op)
		}

		fresh := &monitoringv1.ServiceMonitor{}
		if err := r.client.Get(context.TODO(), r.AutoscalerName(ca), fresh); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		fresh.ResourceVersion = expected.ResourceVersion
		// TODO: find a better way to handle this. Added because of https://github.com/kubernetes-sigs/controller-runtime/pull/2633
		fresh.TypeMeta = expected.TypeMeta
		if !equality.Semantic.DeepEqual(fresh, expected) {
			t.Errorf("expected: %v, got: %v", expected, fresh)
		}
	}
}

func TestCreateOrUpdateAutoscalerPrometheusRule(t *testing.T) {
	r := newFakeReconciler()
	ca := NewClusterAutoscaler()

	expected := r.AutoscalerPrometheusRule(ca)
	if err := controllerutil.SetControllerReference(ca, expected, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	modified := expected.DeepCopy()
	if err := controllerutil.SetControllerReference(ca, modified, r.scheme); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	modified.Spec.Groups[0].Rules = []monitoringv1.Rule{}

	testCases := []struct {
		current    *monitoringv1.PrometheusRule
		expectedOP controllerutil.OperationResult
	}{
		{
			current:    nil,
			expectedOP: controllerutil.OperationResultCreated,
		},
		{
			current:    modified,
			expectedOP: controllerutil.OperationResultUpdated,
		},
	}

	for _, tc := range testCases {
		var r *Reconciler
		if tc.current != nil {
			r = newFakeReconciler(tc.current)
		} else {
			r = newFakeReconciler()
		}

		op, err := r.createOrUpdateAutoscalerPrometheusRule(ca)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if op != tc.expectedOP {
			t.Errorf("expected: %s, got: %s", tc.expectedOP, op)
		}

		fresh := &monitoringv1.PrometheusRule{}
		if err := r.client.Get(context.TODO(), r.AutoscalerName(ca), fresh); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		fresh.ResourceVersion = expected.ResourceVersion
		// TODO: find a better way to handle this. Added because of https://github.com/kubernetes-sigs/controller-runtime/pull/2633
		fresh.TypeMeta = expected.TypeMeta
		if !equality.Semantic.DeepEqual(fresh, expected) {
			t.Errorf("expected: %v, got: %v", expected, fresh)
		}
	}
}
