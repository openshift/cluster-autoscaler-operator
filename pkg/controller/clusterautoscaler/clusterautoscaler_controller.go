package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	autoscalingv1alpha1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/operator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	criticalPod      = "scheduler.alpha.kubernetes.io/critical-pod"
	caServiceAccount = "cluster-autoscaler"
)

// Add creates a new ClusterAutoscaler Controller and adds it to the
// Manager. The Manager will set fields on the Controller and Start it
// when the Manager is Started.
func Add(mgr manager.Manager, cfg *operator.Config) error {
	r := newReconciler(mgr)

	r.SetImage(cfg.ClusterAutoscalerImage)
	r.SetReplicas(cfg.ClusterAutoscalerReplicas)
	r.SetNamespace(cfg.ClusterAutoscalerNamespace)

	return add(mgr, cfg, r)
}

// newReconciler returns a new Reconciler.
func newReconciler(mgr manager.Manager) *Reconciler {
	return &Reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// processEvent is used in predicate functions.  It inspects metadata and
// returns a boolean value representing whether the controller should process an
// event for the object.
func processEvent(meta metav1.Object, cfg *operator.Config) bool {
	// Only process events for objects matching the configured resource name.
	if meta.GetName() != cfg.ClusterAutoscalerName {
		glog.Warningf("Not processing ClusterAutoscaler %s", meta.GetName())
		return false
	}

	return true
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, cfg *operator.Config, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusterautoscaler-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// ClusterAutoscaler is effectively a singleton resource.  A
	// deployment is only created if an instance is found matching the
	// name set at runtime.
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return processEvent(e.Meta, cfg)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processEvent(e.MetaNew, cfg)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processEvent(e.Meta, cfg)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processEvent(e.Meta, cfg)
		},
	}

	// Watch for changes to primary resource ClusterAutoscaler
	err = c.Watch(&source.Kind{Type: &autoscalingv1alpha1.ClusterAutoscaler{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources owned by a ClusterAutoscaler
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &autoscalingv1alpha1.ClusterAutoscaler{},
	})

	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a ClusterAutoscaler object
type Reconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	// The namespace for cluster-autoscaler deployments.
	caNamespace string

	// The cluster-autoscaler image to use in deployments.
	caImage string

	// The number of replicas in cluster-autoscaler deployments.
	caReplicas int32
}

// Reconcile reads that state of the cluster for a ClusterAutoscaler
// object and makes changes based on the state read and what is in the
// ClusterAutoscaler.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Reconciling ClusterAutoscaler %s\n", request.Name)

	// Fetch the ClusterAutoscaler instance
	ca := &autoscalingv1alpha1.ClusterAutoscaler{}
	err := r.client.Get(context.TODO(), request.NamespacedName, ca)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request.  Owned objects are automatically
			// garbage collected. For additional cleanup logic use
			// finalizers.  Return and don't requeue.
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		glog.Errorf("Error reading ClusterAutoscaler: %v", err)
		return reconcile.Result{}, err
	}

	_, err = r.GetAutoscaler(ca)
	if err != nil && !errors.IsNotFound(err) {
		glog.Errorf("Error getting cluster-autoscaler deployment: %v", err)
		return reconcile.Result{}, err
	}

	if errors.IsNotFound(err) {
		if err := r.CreateAutoscaler(ca); err != nil {
			glog.Errorf("Error creating cluster-autoscaler deployment: %v", err)
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	if err := r.UpdateAutoscaler(ca); err != nil {
		glog.Errorf("Error updating cluster-autoscaler deployment: %v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetNamespace sets the cluster-autoscaler namespace on the reconciler.
func (r *Reconciler) SetNamespace(namespace string) {
	r.caNamespace = namespace
}

// SetImage sets the cluster-autoscaler image on the reconciler.
func (r *Reconciler) SetImage(image string) {
	r.caImage = image
}

// SetReplicas sets the number of cluster-autoscaler replicas on the reconciler.
func (r *Reconciler) SetReplicas(replicas int32) {
	r.caReplicas = replicas
}

// CreateAutoscaler will create the deployment for the given the
// ClusterAutoscaler custom resource instance.
func (r *Reconciler) CreateAutoscaler(ca *autoscalingv1alpha1.ClusterAutoscaler) error {
	glog.Infof("Creating cluster-autoscaler deployment %s/%s\n", r.caNamespace, ca.Name)

	deployment := r.AutoscalerDeployment(ca)

	// Set ClusterAutoscaler instance as the owner and controller.
	if err := controllerutil.SetControllerReference(ca, deployment, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), deployment)
}

// UpdateAutoscaler will retrieve the deployment for the given ClusterAutoscaler
// custom resource instance and update it to match the expected spec if needed.
func (r *Reconciler) UpdateAutoscaler(ca *autoscalingv1alpha1.ClusterAutoscaler) error {
	existingDeployment, err := r.GetAutoscaler(ca)
	if err != nil {
		return err
	}

	existingSpec := existingDeployment.Spec.Template.Spec
	expectedSpec := r.AutoscalerPodSpec(ca)

	// Only comparing podSpec for now.
	if equality.Semantic.DeepEqual(existingSpec, expectedSpec) {
		return nil
	}

	existingDeployment.Spec.Template.Spec = *expectedSpec
	return r.client.Update(context.TODO(), existingDeployment)
}

// GetAutoscaler will return the deployment for the given ClusterAutoscaler
// custom resource instance.
func (r *Reconciler) GetAutoscaler(ca *autoscalingv1alpha1.ClusterAutoscaler) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	nn := r.AutoscalerName(ca)

	if err := r.client.Get(context.TODO(), nn, deployment); err != nil {
		return nil, err
	}

	return deployment, nil
}

// AutoscalerName returns the expected NamespacedName for the deployment
// belonging to the given ClusterAutoscaler.
func (r *Reconciler) AutoscalerName(ca *autoscalingv1alpha1.ClusterAutoscaler) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("cluster-autoscaler-%s", ca.Name),
		Namespace: r.caNamespace,
	}
}

// AutoscalerDeployment returns the expected deployment belonging to the given
// ClusterAutoscaler.
func (r *Reconciler) AutoscalerDeployment(ca *autoscalingv1alpha1.ClusterAutoscaler) *appsv1.Deployment {
	namespacedName := r.AutoscalerName(ca)

	labels := map[string]string{
		"cluster-autoscaler": ca.Name,
		"app":                "cluster-autoscaler",
	}

	annotations := map[string]string{
		criticalPod: "",
	}

	podSpec := r.AutoscalerPodSpec(ca)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r.caReplicas,
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

	return deployment
}

// AutoscalerPodSpec returns the expected podSpec for the deployment belonging
// to the given ClusterAutoscaler.
func (r *Reconciler) AutoscalerPodSpec(ca *autoscalingv1alpha1.ClusterAutoscaler) *corev1.PodSpec {
	args := AutoscalerArgs(ca, r.caNamespace)

	spec := &corev1.PodSpec{
		ServiceAccountName: caServiceAccount,
		Containers: []corev1.Container{
			{
				Name:    "cluster-autoscaler",
				Image:   r.caImage,
				Command: []string{"cluster-autoscaler"},
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
