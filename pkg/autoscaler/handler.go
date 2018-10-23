package autoscaler

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	caImage          = "quay.io/bison/cluster-autoscaler:a554b4f5"
	criticalPod      = "scheduler.alpha.kubernetes.io/critical-pod"
	caServiceAccount = "cluster-autoscaler"

	minSizeLabel = "sigs.k8s.io/cluster-api-autoscaler-node-group-min-size"
	maxSizeLabel = "sigs.k8s.io/cluster-api-autoscaler-node-group-max-size"
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
	metrics *Metrics
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.ClusterAutoscaler:
		clusterAutoscaler := o

		// Ignore deletes.  Resources should have their OwnerReference
		// set appropriately which will allow them to be garbage
		// collected automatically.
		if event.Deleted {
			return nil
		}

		dep := autoscalerDeployment(clusterAutoscaler)
		err := sdk.Create(dep)
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create autoscaler deployment: %v", err)
		}

		if errors.IsAlreadyExists(err) {
			return updateAutoscaler(clusterAutoscaler)
		}

		// TODO: Update ClusterAutoscaler status.

	case *v1alpha1.MachineAutoscaler:
		if event.Deleted {
			return h.deleteMachine(o)
		}

		return h.updateMachine(o)
	}

	return nil
}

func (h *Handler) deleteMachine(ma *v1alpha1.MachineAutoscaler) error {
	target, err := machineTarget(ma)
	if err != nil {
		return err
	}

	// TODO(bison): Patch instead of update.
	labels := target.GetLabels()
	delete(labels, minSizeLabel)
	delete(labels, maxSizeLabel)
	target.SetLabels(labels)

	return sdk.Update(target)
}

func (h *Handler) updateMachine(ma *v1alpha1.MachineAutoscaler) error {
	target, err := machineTarget(ma)
	if err != nil {
		return err
	}

	newLabels := map[string]string{
		minSizeLabel: strconv.Itoa(int(ma.Spec.MinReplicas)),
		maxSizeLabel: strconv.Itoa(int(ma.Spec.MaxReplicas)),
	}

	labels := target.GetLabels()

	if machineNeedsUpdate(labels, newLabels) {
		// TODO(bison): We should just patch here.
		labels[minSizeLabel] = newLabels[minSizeLabel]
		labels[maxSizeLabel] = newLabels[maxSizeLabel]

		target.SetLabels(labels)
		return sdk.Update(target)
	}

	return nil
}

func updateAutoscaler(ca *v1alpha1.ClusterAutoscaler) error {
	dep := autoscalerDeployment(ca)
	err := sdk.Get(dep)
	if err != nil {
		return fmt.Errorf("failed to get autoscaler deployment: %v", err)
	}

	podSpec := autoscalerPodSpec(ca)
	if !reflect.DeepEqual(dep.Spec.Template.Spec, podSpec) {
		dep.Spec.Template.Spec = *podSpec
		err = sdk.Update(dep)
		if err != nil {
			return fmt.Errorf("failed to update autoscaler deployment: %v", err)
		}
	}

	return nil
}

func autoscalerDeployment(ca *v1alpha1.ClusterAutoscaler) *appsv1.Deployment {
	var replicas int32 = 1

	deploymentName := fmt.Sprintf("cluster-autoscaler-%s", ca.Name)

	labels := map[string]string{
		"cluster-autoscaler": ca.Name,
		"app":                "cluster-autoscaler",
	}

	annotations := map[string]string{
		criticalPod: "",
	}

	podSpec := autoscalerPodSpec(ca)

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: ca.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: *podSpec,
			},
		},
	}

	addOwnerRefToObject(dep, asOwner(ca))

	return dep
}

func autoscalerPodSpec(ca *v1alpha1.ClusterAutoscaler) *corev1.PodSpec {
	args := AutoscalerArgs(ca)

	spec := &corev1.PodSpec{
		ServiceAccountName: caServiceAccount,
		Containers: []corev1.Container{
			{
				Name:    "cluster-autoscaler",
				Image:   caImage,
				Command: []string{"/cluster-autoscaler"},
				Args:    args,
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: corev1.TolerationOpExists,
			},
		},
	}

	return spec
}

// addOwnerRefToObject appends the desired OwnerReference to the object.
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// asOwner returns an OwnerReference set as the ClusterAutoscaler CR.
func asOwner(ca *v1alpha1.ClusterAutoscaler) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: ca.APIVersion,
		Kind:       ca.Kind,
		Name:       ca.Name,
		UID:        ca.UID,
		Controller: &trueVar,
	}
}

func machineTarget(ma *v1alpha1.MachineAutoscaler) (*unstructured.Unstructured, error) {
	ref := ma.Spec.ScaleTargetRef

	// TODO(bison): Keep a list of supported GVKs?
	if ref.APIVersion != "cluster.k8s.io/v1alpha1" || ref.Kind != "MachineSet" {
		return nil, fmt.Errorf("unsupported scale target: %#v", ref)
	}

	// TODO(bison): We could probably have the SDK do this for us.
	client, _, err := k8sclient.GetResourceClient(ref.APIVersion, ref.Kind, ma.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting resource client: %v", err)
	}

	target, err := client.Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting scale target: %v", err)
	}

	return target, nil
}

func machineNeedsUpdate(have, want map[string]string) bool {
	minDiff := have[minSizeLabel] != want[minSizeLabel]
	maxDiff := have[maxSizeLabel] != want[maxSizeLabel]

	return minDiff || maxDiff
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
