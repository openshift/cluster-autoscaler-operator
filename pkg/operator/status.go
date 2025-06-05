package operator

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	osconfig "github.com/openshift/client-go/config/clientset/versioned"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"
	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Reason messages used in status conditions.
const (
	ReasonAsExpected        = "AsExpected"
	ReasonMissingDependency = "MissingDependency"
	ReasonSyncing           = "SyncingResources"
	ReasonCheckAutoscaler   = "UnableToCheckAutoscalers"
)

const (
	// DegradedCountThreshold is the number of consecutive observed failures (or non-OK statuses)
	// required before the operator reports itself as Degraded in the ClusterOperator status.
	// This helps prevent flapping due to transient issues.
	// At most DegradedCountThreshold * interval seconds will pass before the operator is reported degraded.
	DegradedCountThreshold = 3
)

// StatusReporter reports the status of the operator to the OpenShift
// cluster-version-operator via ClusterOperator resource status.
type StatusReporter struct {
	client                   client.Client
	configClient             osconfig.Interface
	config                   *StatusReporterConfig
	configInformers          configv1informers.SharedInformerFactory
	queue                    workqueue.RateLimitingInterface
	degradedConsecutiveCount int
}

// StatusReporterConfig represents the configuration of a given StatusReporter.
type StatusReporterConfig struct {
	ClusterAutoscalerName      string
	ClusterAutoscalerNamespace string
	ReleaseVersion             string
	RelatedObjects             []configv1.ObjectReference
}

// NewStatusReporter returns a new StatusReporter instance.
func NewStatusReporter(mgr manager.Manager, cfg *StatusReporterConfig) (*StatusReporter, error) {
	var err error

	reporter := &StatusReporter{
		client: mgr.GetClient(),
		config: cfg,
	}

	// Create a client for OpenShift config objects.
	reporter.configClient, err = osconfig.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	reporter.configInformers = configv1informers.NewSharedInformerFactory(reporter.configClient, 10*time.Minute)
	reporter.queue = workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
		Name: "status-report",
	})

	return reporter, nil
}

// SetReleaseVersion sets the configured release version.
func (r *StatusReporter) SetReleaseVersion(version string) {
	r.config.ReleaseVersion = version
}

// SetRelatedObjects sets the configured related objects.
func (r *StatusReporter) SetRelatedObjects(objs []configv1.ObjectReference) {
	r.config.RelatedObjects = objs
}

// AddRelatedObjects adds to the list of related objects.
func (r *StatusReporter) AddRelatedObjects(objs []configv1.ObjectReference) {
	for _, obj := range objs {
		r.config.RelatedObjects = append(r.config.RelatedObjects, obj)
	}
}

// GetClusterOperator fetches the the operator's ClusterOperator object.
func (r *StatusReporter) GetClusterOperator() (*configv1.ClusterOperator, error) {
	return r.configClient.ConfigV1().ClusterOperators().Get(context.Background(), OperatorName, metav1.GetOptions{})
}

// GetOrCreateClusterOperator gets, or if necessary, creates the
// operator's ClusterOperator object and returns it.
func (r *StatusReporter) GetOrCreateClusterOperator() (*configv1.ClusterOperator, error) {
	clusterOperator := &configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: OperatorName,
		},
	}

	existing, err := r.GetClusterOperator()

	if errors.IsNotFound(err) {
		return r.configClient.ConfigV1().ClusterOperators().Create(context.Background(), clusterOperator, metav1.CreateOptions{})
	}

	return existing, err
}

// ApplyStatus applies the given ClusterOperator status to the operator's
// ClusterOperator object if necessary.  The currently configured RelatedObjects
// are automatically set on the status.  If no ClusterOperator objects exists,
// one is created.
func (r *StatusReporter) ApplyStatus(status configv1.ClusterOperatorStatus) error {
	var modified bool

	co, err := r.GetOrCreateClusterOperator()
	if err != nil {
		return err
	}

	// There currently is no circumstance that prevents the operator from
	// upgrading, so we always set OperatorUpgradeable to true here.
	upgradeable := configv1.ClusterOperatorStatusCondition{
		Type:   configv1.OperatorUpgradeable,
		Status: configv1.ConditionTrue,
	}

	v1helpers.SetStatusCondition(&status.Conditions, upgradeable)

	// Set the currently configured related objects.
	status.RelatedObjects = r.config.RelatedObjects

	// If no versions were set explicitly, continue reporting previous versions.
	if status.Versions == nil {
		status.Versions = co.Status.Versions
	}

	// Update LastTransitionTime for all conditions if necessary.
	for i := range status.Conditions {
		condType := status.Conditions[i].Type
		timestamp := metav1.NewTime(time.Now())

		c := v1helpers.FindStatusCondition(co.Status.Conditions, condType)

		// If found, and status doesn't match, update.
		if c != nil && c.Status != status.Conditions[i].Status {
			status.Conditions[i].LastTransitionTime = timestamp
		}

		// If found, and status matches, copy previous.
		if c != nil && c.Status == status.Conditions[i].Status {
			status.Conditions[i].LastTransitionTime = c.LastTransitionTime
		}

		// If it's still nil, update it.
		if status.Conditions[i].LastTransitionTime.IsZero() {
			status.Conditions[i].LastTransitionTime = timestamp
		}
	}

	// If any versions have changed, we need to reset the transition time on the
	// Progressing condition whether or not we actually had any work to do.
	if !equality.Semantic.DeepEqual(status.Versions, co.Status.Versions) {
		util.ResetProgressingTime(&status.Conditions)
	}

	// Copy the current ClusterOperator and overwrite the status.
	requiredCO := &configv1.ClusterOperator{}
	co.DeepCopyInto(requiredCO)
	requiredCO.Status = status

	ensureClusterOperatorStatus(&modified, &co.Status, requiredCO.Status)

	if modified {
		_, err := r.configClient.ConfigV1().ClusterOperators().UpdateStatus(context.Background(), co, metav1.UpdateOptions{})
		return err
	}

	return nil
}

// Copy from CVO while moving to library-go to keep changes scoped and keep current behaviour.
// https://github.com/openshift/cluster-version-operator/blob/1fd0041275414266157bf257043fa402f3bc9ebf/lib/resourcemerge/os.go#L17
func ensureClusterOperatorStatus(modified *bool, existing *configv1.ClusterOperatorStatus, required configv1.ClusterOperatorStatus) {
	if !equality.Semantic.DeepEqual(existing.Conditions, required.Conditions) {
		*modified = true
		existing.Conditions = required.Conditions
	}

	if !equality.Semantic.DeepEqual(existing.Versions, required.Versions) {
		*modified = true
		existing.Versions = required.Versions
	}
	if !equality.Semantic.DeepEqual(existing.Extension.Raw, required.Extension.Raw) {
		*modified = true
		existing.Extension.Raw = required.Extension.Raw
	}
	if !equality.Semantic.DeepEqual(existing.Extension.Object, required.Extension.Object) {
		*modified = true
		existing.Extension.Object = required.Extension.Object
	}
	if !equality.Semantic.DeepEqual(existing.RelatedObjects, required.RelatedObjects) {
		*modified = true
		existing.RelatedObjects = required.RelatedObjects
	}
}

// available reports the operator as available, not progressing, and
// not degraded -- optionally setting a reason and message.  This will
// update the reported operator version.  It should only be called if
// the operands are fully updated and available.
func (r *StatusReporter) available(reason, message string) error {
	status := configv1.ClusterOperatorStatus{
		Conditions: []configv1.ClusterOperatorStatusCondition{
			{
				Type:    configv1.OperatorAvailable,
				Status:  configv1.ConditionTrue,
				Reason:  reason,
				Message: message,
			},
			{
				Type:   configv1.OperatorProgressing,
				Status: configv1.ConditionFalse,
			},
			{
				Type:   configv1.OperatorDegraded,
				Status: configv1.ConditionFalse,
			},
		},
		Versions: []configv1.OperandVersion{
			{
				Name:    "operator",
				Version: r.config.ReleaseVersion,
			},
		},
	}

	klog.Infof("Operator status available: %s", message)

	return r.ApplyStatus(status)
}

// degraded reports the operator as degraded but available, and not
// progressing -- optionally setting a reason and message.
// The operator will not actually be reported as degraded until
// the DegradedCountThreshold is reached, in order to avoid
// flapping the status in the event of transient errors.
func (r *StatusReporter) degraded(reason, message string) error {
	status := configv1.ClusterOperatorStatus{
		Conditions: []configv1.ClusterOperatorStatusCondition{
			{
				Type:   configv1.OperatorAvailable,
				Status: configv1.ConditionTrue,
			},
			{
				Type:   configv1.OperatorProgressing,
				Status: configv1.ConditionFalse,
			},
			{
				Type:    configv1.OperatorDegraded,
				Status:  configv1.ConditionTrue,
				Reason:  reason,
				Message: message,
			},
		},
	}

	r.degradedConsecutiveCount++
	if r.degradedConsecutiveCount >= DegradedCountThreshold {
		klog.Warningf("Operator status degraded: %s", message)
		r.degradedConsecutiveCount = DegradedCountThreshold // Make sure there is no overflow.
		return r.ApplyStatus(status)
	}

	klog.Warningf("Operator status degraded but not yet reported: %s (consecutive count: %d)", message, r.degradedConsecutiveCount)

	return r.available(ReasonAsExpected, "available, but observed potential degradations.")
}

// progressing reports the operator as progressing but available, and not
// degraded -- optionally setting a reason and message.
func (r *StatusReporter) progressing(reason, message string) error {
	status := configv1.ClusterOperatorStatus{
		Conditions: []configv1.ClusterOperatorStatusCondition{
			{
				Type:   configv1.OperatorAvailable,
				Status: configv1.ConditionTrue,
			},
			{
				Type:    configv1.OperatorProgressing,
				Status:  configv1.ConditionTrue,
				Reason:  reason,
				Message: message,
			},
			{
				Type:   configv1.OperatorDegraded,
				Status: configv1.ConditionFalse,
			},
		},
	}

	klog.Infof("Operator status progressing: %s", message)

	return r.ApplyStatus(status)
}

func (c *StatusReporter) processNextWorkItem() {
	key, quit := c.queue.Get()
	if quit {
		return
	}
	defer c.queue.Done(key)
	_, err := c.ReportStatus()
	if err != nil {
		klog.Errorf("status reporting failed: %v", err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
}

func (c *StatusReporter) runWorker(queueCtx context.Context) {
	wait.UntilWithContext(
		queueCtx,
		func(queueCtx context.Context) {
			for {
				select {
				case <-queueCtx.Done():
					return
				default:
					c.processNextWorkItem()
				}
			}
		},
		1*time.Second)
}

// Start checks the status of dependencies and reports the operator's status. It
// will poll until stopCh is closed or prerequisites are satisfied, in which
// case it will report the operator as available the configured version and wait
// for stopCh to close before returning.
func (r *StatusReporter) Start(stop context.Context) error {
	// run informer to make sure that we report status everytime the cluster operator changes
	// for example. if some external process override message or reason.
	klog.Info("Starting status reporter: cluster operator informers")
	go r.configInformers.Start(stop.Done())

	cacheWaitCtx, cacheWaitCancel := context.WithTimeout(stop, 30*time.Second)
	defer cacheWaitCancel()

	operatorInformer := r.configInformers.Config().V1().ClusterOperators().Informer()
	if !cache.WaitForCacheSync(cacheWaitCtx.Done(), operatorInformer.HasSynced) {
		return fmt.Errorf("unable to sync clusteroperators informer")
	}

	klog.Info("Starting status reporter: worker")
	go r.runWorker(stop)

	operatorInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.queue.Add("cluster")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.queue.Add("cluster")
		},
		DeleteFunc: func(obj interface{}) {
			r.queue.Add("cluster")
		},
	})

	interval := 15 * time.Second

	// Poll the status of our prerequisites and set our status accordingly.
	// Rather than return errors and stop polling, errors here should just be
	// reported in the status message or logged.
	pollFunc := func() (bool, error) {
		r.queue.Add("cluster")
		return false, nil
	}
	err := wait.PollImmediateUntil(interval, pollFunc, stop.Done())

	// Block until the stop channel is closed.
	<-stop.Done()

	return err
}

// ReportStatus checks the status of each dependency and operand and reports the
// appropriate status via the operator's ClusterOperator object.
func (r *StatusReporter) ReportStatus() (bool, error) {
	// Check that the machine-api-operator is reporting available.
	ok, err := r.CheckMachineAPI()
	if err != nil {
		msg := fmt.Sprintf("error checking machine-api status: %v", err)
		return false, r.degraded(ReasonMissingDependency, msg)
	}

	if !ok {
		return false, r.degraded(ReasonMissingDependency, "machine-api not ready")
	}

	// Check that any CluterAutoscaler deployments are updated and available.
	ok, err = r.CheckClusterAutoscaler()
	if err != nil {
		msg := fmt.Sprintf("error checking autoscaler status: %v", err)
		return false, r.degraded(ReasonCheckAutoscaler, msg)
	}

	// Reset the degraded consecutive count since we have not observed degraded.
	r.degradedConsecutiveCount = 0

	if !ok {
		msg := fmt.Sprintf("updating to %s", r.config.ReleaseVersion)
		return false, r.progressing(ReasonSyncing, msg)
	}

	msg := fmt.Sprintf("at version %s", r.config.ReleaseVersion)
	if err := r.available(ReasonAsExpected, msg); err != nil {
		return false, err
	}

	return true, nil
}

// CheckMachineAPI checks the status of the machine-api-operator as
// reported to the CVO.  It returns true if the operator is available
// and not degraded.
func (r *StatusReporter) CheckMachineAPI() (bool, error) {
	mao, err := r.configClient.ConfigV1().ClusterOperators().
		Get(context.Background(), "machine-api", metav1.GetOptions{})

	if err != nil {
		klog.Errorf("failed to get dependency machine-api status: %v", err)
		return false, err
	}

	conds := mao.Status.Conditions

	if v1helpers.IsStatusConditionTrue(conds, configv1.OperatorAvailable) &&
		v1helpers.IsStatusConditionFalse(conds, configv1.OperatorDegraded) {
		return true, nil
	}

	klog.Infof("machine-api-operator not ready yet")
	return false, nil
}

// CheckClusterAutoscaler checks the status of any cluster-autoscaler
// deployments. It returns a bool indicating whether the deployments are
// available and fully updated to the latest version and an error.
func (r *StatusReporter) CheckClusterAutoscaler() (bool, error) {
	ca := &autoscalingv1.ClusterAutoscaler{}
	caName := client.ObjectKey{Name: r.config.ClusterAutoscalerName}

	if err := r.client.Get(context.TODO(), caName, ca); err != nil {
		if errors.IsNotFound(err) {
			klog.Info("No ClusterAutoscaler. Reporting available.")
			return true, nil
		}

		klog.Errorf("Error getting ClusterAutoscaler: %v", err)
		return false, err
	}

	deployment := &appsv1.Deployment{}
	deploymentName := client.ObjectKey{
		Name:      fmt.Sprintf("%s-%s", OperatorName, r.config.ClusterAutoscalerName),
		Namespace: r.config.ClusterAutoscalerNamespace,
	}

	if err := r.client.Get(context.TODO(), deploymentName, deployment); err != nil {
		if errors.IsNotFound(err) {
			klog.Info("No ClusterAutoscaler deployment. Reporting unavailable.")
			return false, nil
		}

		klog.Errorf("Error getting ClusterAutoscaler deployment: %v", err)
		return false, err
	}

	if !util.ReleaseVersionMatches(deployment, r.config.ReleaseVersion) {
		klog.Info("ClusterAutoscaler deployment version not current.")
		return false, nil
	}

	if !util.DeploymentUpdated(deployment) {
		klog.Info("ClusterAutoscaler deployment updating.")
		return false, nil
	}

	klog.Info("ClusterAutoscaler deployment is available and updated.")

	return true, nil
}
