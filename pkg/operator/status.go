package operator

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	configv1 "github.com/openshift/api/config/v1"
	osconfig "github.com/openshift/client-go/config/clientset/versioned"
	cvorm "github.com/openshift/cluster-version-operator/lib/resourcemerge"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// Reason messages used in status conditions.
const (
	ReasonEmpty             = ""
	ReasonMissingDependency = "MissingDependency"
	ReasonCheckAutoscaler   = "UnableToCheckAutoscalers"
)

// StatusReporter reports the status of the operator to the OpenShift
// cluster-version-operator via ClusterOperator resource status.
type StatusReporter struct {
	client         osconfig.Interface
	relatedObjects []configv1.ObjectReference
	releaseVersion string
}

// NewStatusReporter returns a new StatusReporter instance.
func NewStatusReporter(cfg *rest.Config, relatedObjects []configv1.ObjectReference, releaseVersion string) (*StatusReporter, error) {
	var err error
	reporter := &StatusReporter{
		relatedObjects: relatedObjects,
	}

	// Create a client for OpenShift config objects.
	reporter.client, err = osconfig.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return reporter, nil
}

// GetOrCreateClusterOperator gets, or if necessary, creates the
// operator's ClusterOperator object and returns it.
func (r *StatusReporter) GetOrCreateClusterOperator() (*configv1.ClusterOperator, error) {
	clusterOperator := &configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: OperatorName,
		},
	}

	existing, err := r.client.ConfigV1().ClusterOperators().
		Get(OperatorName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		return r.client.ConfigV1().ClusterOperators().Create(clusterOperator)
	}

	return existing, err
}

// ApplyConditions applies the given conditions to the ClusterOperator
// resource's status.
func (r *StatusReporter) ApplyConditions(conds []configv1.ClusterOperatorStatusCondition, reachedLevel bool) error {
	status := configv1.ClusterOperatorStatus{
		RelatedObjects: r.relatedObjects,
	}

	for _, c := range conds {
		cvorm.SetOperatorStatusCondition(&status.Conditions, c)
	}

	if reachedLevel {
		if len(r.releaseVersion) > 0 {
			status.Versions = []configv1.OperandVersion{
				{
					Name:    "operator",
					Version: r.releaseVersion,
				},
			}
		} else {
			status.Versions = nil
		}
	}

	co, err := r.GetOrCreateClusterOperator()
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(co.Status, status) {
		co.Status = status
		_, err = r.client.ConfigV1().ClusterOperators().UpdateStatus(co)
	}

	return err
}

// Available reports the operator as available, not progressing, and
// not failing -- optionally setting a reason and message.
func (r *StatusReporter) Available(reason, message string) error {
	conditions := []configv1.ClusterOperatorStatusCondition{
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
			Type:   configv1.OperatorFailing,
			Status: configv1.ConditionFalse,
		},
	}
	return r.ApplyConditions(conditions, true)
}

// Fail reports the operator as failing but available, and not
// progressing -- optionally setting a reason and message.
func (r *StatusReporter) Fail(reason, message string) error {
	conditions := []configv1.ClusterOperatorStatusCondition{
		{
			Type:   configv1.OperatorAvailable,
			Status: configv1.ConditionTrue,
		},
		{
			Type:   configv1.OperatorProgressing,
			Status: configv1.ConditionFalse,
		},
		{
			Type:    configv1.OperatorFailing,
			Status:  configv1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
	}

	return r.ApplyConditions(conditions, false)
}

type AvailableChecker interface {
	// AvailableAndUpdated returns true if the reconciler reports all
	// cluster autoscalers are at the latest version.
	AvailableAndUpdated() (bool, error)
}

// Report checks the status of dependencies and reports the operator's
// status. It will poll until stopCh is closed or prerequisites are
// satisfied, in which case it will report the operator as available
// and return. check is used to verify that the reconciler has reached
// the desired state.
func (r *StatusReporter) Report(stopCh <-chan struct{}, check AvailableChecker) error {
	interval := 15 * time.Second

	// Poll the status of our prerequisites and set our status
	// accordingly.  Rather than return errors and stop polling, most
	// errors here should just be reported in the status message.
	pollFunc := func() (bool, error) {
		ok, err := r.CheckMachineAPI()
		if err != nil {
			r.Fail(ReasonMissingDependency, fmt.Sprintf("error checking machine-api operator status %v", err))
			return false, nil
		}

		if !ok {
			r.Fail(ReasonMissingDependency, "machine-api operator not ready")
			return false, nil
		}

		ok, err = check.AvailableAndUpdated()
		if err != nil {
			r.Fail(ReasonCheckAutoscaler, fmt.Sprintf("error checking autoscaler operator status %v", err))
			return false, nil
		}
		if !ok {
			// TODO: technically we are progressing, report that here
			return false, nil
		}

		r.Available(ReasonEmpty, "")
		return true, nil
	}

	return wait.PollImmediateUntil(interval, pollFunc, stopCh)
}

// CheckMachineAPI checks the status of the machine-api-operator as
// reported to the CVO.  It returns true if the operator is available
// and not failing.
func (r *StatusReporter) CheckMachineAPI() (bool, error) {
	mao, err := r.client.ConfigV1().ClusterOperators().
		Get("machine-api", metav1.GetOptions{})

	if err != nil {
		glog.Errorf("failed to get dependency machine-api status: %v", err)
		return false, err
	}

	conds := mao.Status.Conditions

	if cvorm.IsOperatorStatusConditionTrue(conds, configv1.OperatorAvailable) &&
		cvorm.IsOperatorStatusConditionFalse(conds, configv1.OperatorFailing) {
		return true, nil
	}

	glog.Infof("machine-api-operator not ready yet")
	return false, nil
}
