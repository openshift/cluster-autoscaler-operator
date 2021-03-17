package clusterautoscaler

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createOrUpdateObjectForCA will ensure an object is created or updated according to the passed f mutate function
func (r *Reconciler) createOrUpdateObjectForCA(ca *autoscalingv1.ClusterAutoscaler, desired metav1.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	if err := controllerutil.SetControllerReference(ca, desired, r.scheme); err != nil {
		return "", err
	}

	ro, ok := desired.(runtime.Object)
	if !ok {
		return "", fmt.Errorf("can not covert %T to a runtime.Object", desired)
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, ro, f)
	if err != nil {
		return "", err
	}

	klog.V(4).Infof("Ensuring object %q of type %T, operation: %v", desired.GetName(), desired, op)
	return op, nil
}

// createOrUpdateAutoscalerService will create or update a service
// for the given ClusterAutoscaler custom resource instance.
func (r *Reconciler) createOrUpdateAutoscalerService(ca *autoscalingv1.ClusterAutoscaler) (controllerutil.OperationResult, error) {
	desired := r.AutoscalerService(ca)
	return r.createOrUpdateObjectForCA(ca, desired, func() error {
		// currentClusterIP is immutable so we need to keep it
		currentClusterIP := desired.Spec.ClusterIP
		desired.Spec = r.AutoscalerService(ca).Spec
		desired.Spec.ClusterIP = currentClusterIP
		return nil
	})
}

// createOrUpdateAutoscalerServiceMonitor will create or update a serviceMonitor
// for the given ClusterAutoscaler custom resource instance.
func (r *Reconciler) createOrUpdateAutoscalerServiceMonitor(ca *autoscalingv1.ClusterAutoscaler) (controllerutil.OperationResult, error) {
	desired := r.AutoscalerServiceMonitor(ca)
	return r.createOrUpdateObjectForCA(ca, desired, func() error {
		desired.Spec = r.AutoscalerServiceMonitor(ca).Spec
		return nil
	})
}

// createOrUpdateAutoscalerPrometheusRule will create or update a prometheusRule
// for the given ClusterAutoscaler custom resource instance.
func (r *Reconciler) createOrUpdateAutoscalerPrometheusRule(ca *autoscalingv1.ClusterAutoscaler) (controllerutil.OperationResult, error) {
	desired := r.AutoscalerPrometheusRule(ca)
	return r.createOrUpdateObjectForCA(ca, desired, func() error {
		desired.Spec = r.AutoscalerPrometheusRule(ca).Spec
		return nil
	})
}

func (r *Reconciler) ensureAutoscalerMonitoring(ca *autoscalingv1.ClusterAutoscaler) error {
	if _, err := r.createOrUpdateAutoscalerService(ca); err != nil {
		return fmt.Errorf("error ensuring cluster autoscaler service: %v", err)
	}

	if _, err := r.createOrUpdateAutoscalerServiceMonitor(ca); err != nil {
		return fmt.Errorf("error ensuring cluster autoscaler serviceMonitor: %v", err)
	}

	if _, err := r.createOrUpdateAutoscalerPrometheusRule(ca); err != nil {
		return fmt.Errorf("error ensuring cluster autoscaler prometheusRule: %v", err)
	}

	return nil
}

func (r *Reconciler) AutoscalerService(ca *autoscalingv1.ClusterAutoscaler) *corev1.Service {
	namespacedName := r.AutoscalerName(ca)
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"k8s-app": "cluster-autoscaler",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					TargetPort: intstr.FromString("metrics"),
					Protocol:   corev1.ProtocolTCP,
					Port:       8085,
				},
			},
			Selector: map[string]string{
				"k8s-app": "cluster-autoscaler",
			},
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}
}

func (r *Reconciler) AutoscalerServiceMonitor(ca *autoscalingv1.ClusterAutoscaler) *monitoringv1.ServiceMonitor {
	namespacedName := r.AutoscalerName(ca)
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       monitoringv1.ServiceMonitorsKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"k8s-app": "cluster-autoscaler",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
					Interval:        "30s",
					Port:            "metrics",
					Scheme:          "http",
				},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{namespacedName.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"k8s-app": "cluster-autoscaler"},
			},
		},
	}
}

func (r *Reconciler) AutoscalerPrometheusRule(ca *autoscalingv1.ClusterAutoscaler) *monitoringv1.PrometheusRule {
	namespacedName := r.AutoscalerName(ca)
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       monitoringv1.PrometheusRuleKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"prometheus": "k8s",
				"role":       "alert-rules",
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "general.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "ClusterAutoscalerUnschedulablePods",
							Expr:  intstr.FromString(fmt.Sprintf("cluster_autoscaler_unschedulable_pods_count{service=\"%s\"} > 0", namespacedName.Name)),
							For:   "20m",
							Labels: map[string]string{
								"severity": "warning",
							},
							Annotations: map[string]string{
								"message": "Cluster Autoscaler has {{ $value }} unschedulable pods",
							},
						},
						{
							Alert: "ClusterAutoscalerNotSafeToScale",
							Expr:  intstr.FromString(fmt.Sprintf("cluster_autoscaler_cluster_safe_to_autoscale{service=\"%s\"} != 1", namespacedName.Name)),
							For:   "15m",
							Labels: map[string]string{
								"severity": "warning",
							},
							Annotations: map[string]string{
								"message": "Cluster Autoscaler is reporting that the cluster is not ready for scaling",
							},
						},
						{
							Alert: "ClusterAutoscalerExcessiveUnregisteredNodeCleanup",
							Expr:  intstr.FromString(fmt.Sprintf("rate(cluster_autoscaler_old_unregistered_nodes_removed_count[6h]) > 1")),
							Labels: map[string]string{
								"severity": "warning",
							},
							Annotations: map[string]string{
								"message": "Cluster Autoscaler has removed more than 1 unregistered node in the last 6 hours",
							},
						},
					},
				},
			},
		},
	}
}
