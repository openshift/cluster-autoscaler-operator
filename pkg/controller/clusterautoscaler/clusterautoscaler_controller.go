package clusterautoscaler

import (
	"context"
	"fmt"
	"os"
	goruntime "runtime"

	configv1 "github.com/openshift/api/config/v1"
	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"k8s.io/klog/v2"
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
	controllerName                 = "cluster_autoscaler_controller"
	caServiceAccount               = "cluster-autoscaler"
	caPriorityClassName            = "system-cluster-critical"
	CAPIGroupEnvVar                = "CAPI_GROUP"
	defaultCAPIGroup               = "machine.openshift.io"
	CAPIScaleZeroDefaultArchEnvVar = "CAPI_SCALE_ZERO_DEFAULT_ARCH"
	CAPIVersionEnvVar              = "CAPI_VERSION"
	defaultCAPIVersion             = "v1beta1"
	infrastructureName             = "cluster"
)

// NewReconciler returns a new Reconciler.
func NewReconciler(mgr manager.Manager, config Config) *Reconciler {
	return &Reconciler{
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		recorder:  mgr.GetEventRecorderFor(controllerName),
		validator: NewValidator(config.Name, mgr.GetClient(), mgr.GetScheme()),
		config:    config,
	}
}

// Config represents the configuration for a reconciler instance.
type Config struct {
	// The release version assigned to the operator config.
	ReleaseVersion string
	// The name of the singleton ClusterAutoscaler resource.
	Name string
	// The namespace for cluster-autoscaler deployments.
	Namespace string
	// The cluster-autoscaler image to use in deployments.
	Image string
	// The number of replicas in cluster-autoscaler deployments.
	Replicas int32
	// The name of the CloudProvider.
	CloudProvider string
	// The log verbosity level for the cluster-autoscaler.
	Verbosity int
	// Additional arguments passed to the cluster-autoscaler.
	ExtraArgs string
	// The provider type for the specific cloud provider of the OpenShift install.
	platformType configv1.PlatformType
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a ClusterAutoscaler object
type Reconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	recorder  record.EventRecorder
	config    Config
	scheme    *runtime.Scheme
	validator *Validator
}

// getCAPIGroup returns a string that specifies the group for the API.
// It will return either the value from the
// CAPI_GROUP environment variable, or the default value i.e cluster.x-k8s.io.
func getCAPIGroup() string {
	g := os.Getenv(CAPIGroupEnvVar)
	if g == "" {
		g = defaultCAPIGroup
	}
	klog.V(4).Infof("Using API Group %q", g)
	return g
}

// getCAPIVersion returns a string the specifies the version for the API.
// It will return either the value from the CAPI_VERSION environment variable,
// or the default value i.e v1beta1
func getCAPIVersion() string {
	v := os.Getenv(CAPIVersionEnvVar)
	if v == "" {
		v = defaultCAPIVersion
	}
	klog.V(4).Infof("Using API Version %q", v)
	return v
}

// AddToManager adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// ClusterAutoscaler is effectively a singleton resource.  A
	// deployment is only created if an instance is found matching the
	// name set at runtime.
	p := predicate.TypedFuncs[*autoscalingv1.ClusterAutoscaler]{
		CreateFunc: func(e event.TypedCreateEvent[*autoscalingv1.ClusterAutoscaler]) bool {
			return r.NamePredicate(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[*autoscalingv1.ClusterAutoscaler]) bool {
			return r.NamePredicate(e.ObjectNew)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[*autoscalingv1.ClusterAutoscaler]) bool {
			return r.NamePredicate(e.Object)
		},
		GenericFunc: func(e event.TypedGenericEvent[*autoscalingv1.ClusterAutoscaler]) bool {
			return r.NamePredicate(e.Object)
		},
	}

	// Watch for changes to primary resource ClusterAutoscaler
	if err := c.Watch(source.Kind(mgr.GetCache(), &autoscalingv1.ClusterAutoscaler{}, &handler.TypedEnqueueRequestForObject[*autoscalingv1.ClusterAutoscaler]{}, p)); err != nil {
		return err
	}

	// Watch for changes to secondary resources owned by a ClusterAutoscaler
	if err := c.Watch(source.Kind(mgr.GetCache(), &appsv1.Deployment{}, handler.TypedEnqueueRequestForOwner[*appsv1.Deployment](
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		&autoscalingv1.ClusterAutoscaler{},
		handler.OnlyControllerOwner(),
	))); err != nil {
		return err
	}

	// Watch for changes to monitoring resources owned by a ClusterAutoscaler
	if err := c.Watch(source.Kind(mgr.GetCache(), &corev1.Service{}, handler.TypedEnqueueRequestForOwner[*corev1.Service](
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		&autoscalingv1.ClusterAutoscaler{},
		handler.OnlyControllerOwner(),
	))); err != nil {
		return err
	}

	if err := c.Watch(source.Kind(mgr.GetCache(), &monitoringv1.ServiceMonitor{}, handler.TypedEnqueueRequestForOwner[*monitoringv1.ServiceMonitor](
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		&autoscalingv1.ClusterAutoscaler{},
		handler.OnlyControllerOwner(),
	))); err != nil {
		return err
	}

	return c.Watch(source.Kind(mgr.GetCache(), &monitoringv1.PrometheusRule{}, handler.TypedEnqueueRequestForOwner[*monitoringv1.PrometheusRule](
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		&autoscalingv1.ClusterAutoscaler{},
		handler.OnlyControllerOwner(),
	)))
}

// Reconcile reads that state of the cluster for a ClusterAutoscaler
// object and makes changes based on the state read and what is in the
// ClusterAutoscaler.Spec
func (r *Reconciler) Reconcile(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
	// TODO(elmiko) update this function to use the context that is provided
	klog.Infof("Reconciling ClusterAutoscaler %s\n", request.Name)

	// Fetch the ClusterAutoscaler instance
	ca := &autoscalingv1.ClusterAutoscaler{}
	err := r.client.Get(context.TODO(), request.NamespacedName, ca)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after
			// reconcile request.  Owned objects are automatically
			// garbage collected. For additional cleanup logic use
			// finalizers.  Return and don't requeue.
			klog.Infof("ClusterAutoscaler %s not found, will not reconcile", request.Name)
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		klog.Errorf("Error reading ClusterAutoscaler: %v", err)
		return reconcile.Result{}, err
	}

	// caRef is a reference to the ClusterAutoscaler object, but with the
	// namespace for cluster-autoscaler deployments set.  This keeps events
	// generated for these cluster scoped objects out of the default namespace.
	caRef := r.objectReference(ca)

	// Validate the ClusterAutoscaler early and requeue if any errors are found.
	if res := r.validator.Validate(ca); !res.IsValid() {
		errMsg := fmt.Sprintf("ClusterAutoscaler validation error: %v", res.Errors)
		r.recorder.Event(caRef, corev1.EventTypeWarning, "FailedValidation", errMsg)
		klog.Error(errMsg)

		return reconcile.Result{}, res.Errors
	}

	existingDeployment, err := r.GetAutoscaler(ca)
	if err != nil && !errors.IsNotFound(err) {
		errMsg := fmt.Sprintf("Error getting cluster-autoscaler deployment: %v", err)
		r.recorder.Event(caRef, corev1.EventTypeWarning, "FailedGetDeployment", errMsg)
		klog.Error(errMsg)

		return reconcile.Result{}, err
	}

	// Make sure not to create a new deployment when the CA is being removed.
	if ca.GetDeletionTimestamp() != nil {
		if !errors.IsNotFound(err) {
			// We've already checked for other errors, so this means there was no error, ie the deployment exists.
			// Remove the deployment if it still exists (GC may have beaten us to this).
			if err := r.client.Delete(context.TODO(), existingDeployment); err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Update the cluster provider type.
	if r.config.platformType == "" {
		platformType, err := r.getPlatformType()
		if err != nil {
			return reconcile.Result{}, err
		}
		r.config.platformType = platformType
	}

	if err := r.ensureAutoscalerMonitoring(ca); err != nil {
		errMsg := fmt.Sprintf("Error ensuring ClusterAutoscaler monitoring: %v", err)
		r.recorder.Event(caRef, corev1.EventTypeWarning, "FailedCreate", errMsg)
		klog.Error(errMsg)

		return reconcile.Result{}, err
	}
	klog.Info("Ensured ClusterAutoscaler monitoring")

	if errors.IsNotFound(err) {
		if err := r.CreateAutoscaler(ca); err != nil {
			errMsg := fmt.Sprintf("Error creating ClusterAutoscaler deployment: %v", err)
			r.recorder.Event(caRef, corev1.EventTypeWarning, "FailedCreate", errMsg)
			klog.Error(errMsg)

			return reconcile.Result{}, err
		}

		msg := fmt.Sprintf("Created ClusterAutoscaler deployment: %s", r.AutoscalerName(ca))
		r.recorder.Eventf(caRef, corev1.EventTypeNormal, "SuccessfulCreate", msg)
		klog.Info(msg)

		return reconcile.Result{}, nil
	}

	if err := r.UpdateAutoscaler(ca); err != nil {
		errMsg := fmt.Sprintf("Error updating cluster-autoscaler deployment: %v", err)
		r.recorder.Event(caRef, corev1.EventTypeWarning, "FailedUpdate", errMsg)
		klog.Error(errMsg)

		return reconcile.Result{}, err
	}

	msg := fmt.Sprintf("Updated ClusterAutoscaler deployment: %s", r.AutoscalerName(ca))
	r.recorder.Eventf(caRef, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
	klog.Info(msg)

	return reconcile.Result{}, nil
}

// Validator returns the validator currently configured for the reconciler.
func (r *Reconciler) Validator() *Validator {
	return r.validator
}

// SetConfig sets the given config on the reconciler.
func (r *Reconciler) SetConfig(cfg Config) {
	r.config = cfg
}

// NamePredicate is used in predicate functions.  It returns true if
// the object's name matches the configured name of the singleton
// ClusterAutoscaler resource.
func (r *Reconciler) NamePredicate(meta metav1.Object) bool {
	// Only process events for objects matching the configured resource name.
	if meta.GetName() != r.config.Name {
		klog.Warningf("Not processing ClusterAutoscaler %s", meta.GetName())
		return false
	}

	return true
}

// CreateAutoscaler will create the deployment for the given the
// ClusterAutoscaler custom resource instance.
func (r *Reconciler) CreateAutoscaler(ca *autoscalingv1.ClusterAutoscaler) error {
	klog.Infof("Creating ClusterAutoscaler deployment: %s\n", r.AutoscalerName(ca))

	deployment := r.AutoscalerDeployment(ca)

	// Set ClusterAutoscaler instance as the owner and controller.
	if err := controllerutil.SetControllerReference(ca, deployment, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), deployment)
}

// UpdateAutoscaler will retrieve the deployment for the given ClusterAutoscaler
// custom resource instance and update it to match the expected spec if needed.
func (r *Reconciler) UpdateAutoscaler(ca *autoscalingv1.ClusterAutoscaler) error {
	existingDeployment, err := r.GetAutoscaler(ca)
	if err != nil {
		return err
	}

	existingSpec := existingDeployment.Spec.Template.Spec
	expectedSpec := r.AutoscalerPodSpec(ca)

	// Only comparing podSpec and release version for now.
	if equality.Semantic.DeepEqual(existingSpec, expectedSpec) &&
		util.ReleaseVersionMatches(ca, r.config.ReleaseVersion) {
		return nil
	}

	existingDeployment.Spec.Template.Spec = *expectedSpec

	r.UpdateAnnotations(existingDeployment)
	r.UpdateAnnotations(&existingDeployment.Spec.Template)

	return r.client.Update(context.TODO(), existingDeployment)
}

// GetAutoscaler will return the deployment for the given ClusterAutoscaler
// custom resource instance.
func (r *Reconciler) GetAutoscaler(ca *autoscalingv1.ClusterAutoscaler) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	nn := r.AutoscalerName(ca)

	if err := r.client.Get(context.TODO(), nn, deployment); err != nil {
		return nil, err
	}

	return deployment, nil
}

// AutoscalerName returns the expected NamespacedName for the deployment
// belonging to the given ClusterAutoscaler.
func (r *Reconciler) AutoscalerName(ca *autoscalingv1.ClusterAutoscaler) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("cluster-autoscaler-%s", ca.Name),
		Namespace: r.config.Namespace,
	}
}

// UpdateAnnotations updates the annotations on the given object to the values
// currently expected by the controller.
func (r *Reconciler) UpdateAnnotations(obj metav1.Object) {
	annotations := obj.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[util.CriticalPodAnnotation] = ""
	annotations[util.ReleaseVersionAnnotation] = r.config.ReleaseVersion

	obj.SetAnnotations(annotations)
}

// AutoscalerDeployment returns the expected deployment belonging to the given
// ClusterAutoscaler.
func (r *Reconciler) AutoscalerDeployment(ca *autoscalingv1.ClusterAutoscaler) *appsv1.Deployment {
	namespacedName := r.AutoscalerName(ca)

	labels := map[string]string{
		"cluster-autoscaler": ca.Name,
		"k8s-app":            "cluster-autoscaler",
	}

	annotations := map[string]string{
		util.CriticalPodAnnotation:        "",
		util.ReleaseVersionAnnotation:     r.config.ReleaseVersion,
		util.WorkloadManagementAnnotation: util.WorkloadManagementSchedulingPreferred,
	}

	podSpec := r.AutoscalerPodSpec(ca)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespacedName.Name,
			Namespace:   namespacedName.Namespace,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r.config.Replicas,
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
func (r *Reconciler) AutoscalerPodSpec(ca *autoscalingv1.ClusterAutoscaler) *corev1.PodSpec {
	args := AutoscalerArgs(ca, &r.config)

	if r.config.ExtraArgs != "" {
		args = append(args, r.config.ExtraArgs)
	}

	capiGroup := getCAPIGroup()
	capiVersion := getCAPIVersion()

	spec := &corev1.PodSpec{
		ServiceAccountName: caServiceAccount,
		PriorityClassName:  caPriorityClassName,
		NodeSelector: map[string]string{
			"node-role.kubernetes.io/master": "",
			"beta.kubernetes.io/os":          "linux",
		},
		Containers: []corev1.Container{
			{
				Name:    "cluster-autoscaler",
				Image:   r.config.Image,
				Command: []string{"cluster-autoscaler"},
				Args:    args,
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 8085,
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  CAPIGroupEnvVar,
						Value: capiGroup,
					},
					{
						Name:  CAPIVersionEnvVar,
						Value: capiVersion,
					},
					{
						Name:  CAPIScaleZeroDefaultArchEnvVar,
						Value: goruntime.GOARCH,
					},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
				},
			},
		},

		Tolerations: []corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: corev1.TolerationOpExists,
			},
			{

				Key:      "node-role.kubernetes.io/master",
				Effect:   corev1.TaintEffectNoSchedule,
				Operator: corev1.TolerationOpExists,
			},
		},
	}

	return spec
}

// objectReference returns a reference to the given object, but will set the
// configured deployment namesapce if no namespace was previously set.  This is
// useful for referencing cluster scoped objects in events without the events
// being created in the default namespace.
func (r *Reconciler) objectReference(obj runtime.Object) *corev1.ObjectReference {
	ref, err := reference.GetReference(r.scheme, obj)
	if err != nil {
		klog.Errorf("Error creating object reference: %v", err)
		return nil
	}

	if ref != nil && ref.Namespace == "" {
		ref.Namespace = r.config.Namespace
	}

	return ref
}

func (r *Reconciler) getPlatformType() (configv1.PlatformType, error) {
	infrastructure := &configv1.Infrastructure{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Name: infrastructureName}, infrastructure); err != nil {
		return "", fmt.Errorf("unable to get infrastructure object: %w", err)
	}

	return infrastructure.Status.PlatformStatus.Type, nil
}
