package autoscaler

import (
	"context"
	"fmt"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewHandler(m *Metrics) sdk.Handler {
	return &Handler{
		metrics: m,
	}
}

type Metrics struct {
	operatorErrors prometheus.Counter
}

type Handler struct {
	// Metrics example
	metrics *Metrics

	// Fill me
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.ClusterAutoscaler:
		err := sdk.Create(newAutoScalerPod(o))
		if err != nil && !errors.IsAlreadyExists(err) {
			logrus.Errorf("failed to create cluster-autoscaler pod : %v", err)
			// increment error metric
			h.metrics.operatorErrors.Inc()
			return err
		}
	}
	return nil
}

func newAutoScalerPod(ca *v1alpha1.ClusterAutoscaler) *corev1.Pod {
	const caImage = "quay.io/bison/cluster-autoscaler:a554b4f5"
	const caServiceAccount = "cluster-autoscaler"

	labels := map[string]string{
		"app": "cluster-autoscaler",
	}

	// TODO: Error handling.
	args, _ := ArgsFromCR(ca)

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-autoscaler-%s", ca.Name),
			Namespace: ca.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ca, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterAutoscaler",
				}),
			},
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: caServiceAccount,
			Containers: []corev1.Container{
				{
					Name:    "cluster-autoscaler",
					Image:   caImage,
					Command: []string{"/cluster-autoscaler"},
					Args:    args,
				},
			},
		},
	}
}

func RegisterOperatorMetrics() (*Metrics, error) {
	operatorErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memcached_operator_reconcile_errors_total",
		Help: "Number of errors that occurred while reconciling the memcached deployment",
	})
	err := prometheus.Register(operatorErrors)
	if err != nil {
		return nil, err
	}
	return &Metrics{operatorErrors: operatorErrors}, nil
}
