package helpers

import (
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
)

// TestDeployment wraps the appsv1.Deployment type to add helper methods.
type TestDeployment struct {
	appsv1.Deployment
}

// NewTestDeployment returns a new TestDeployment wrapping the given
// appsv1.Deployment object.
func NewTestDeployment(dep *appsv1.Deployment) *TestDeployment {
	return &TestDeployment{Deployment: *dep}
}

// DeploymentCopy returns a deep copy of the wrapped appsv1.Deployment object.
func (d *TestDeployment) DeploymentCopy() *appsv1.Deployment {
	newDeployment := &appsv1.Deployment{}
	d.Deployment.DeepCopyInto(newDeployment)

	return newDeployment
}

// WithAvailableReplicas returns a copy of the wrapped appsv1.Deployment object
// with the AvailableReplicas set to the given value.
func (d *TestDeployment) WithAvailableReplicas(n int32) *appsv1.Deployment {
	newDeployment := d.DeploymentCopy()
	newDeployment.Status.AvailableReplicas = n

	return newDeployment
}

// WithReleaseVersion returns a copy of the wrapped appsv1.Deployment object
// with the release version annotation set to the given value.
func (d *TestDeployment) WithReleaseVersion(v string) *appsv1.Deployment {
	newDeployment := d.DeploymentCopy()
	annotations := newDeployment.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[util.ReleaseVersionAnnotation] = v
	newDeployment.SetAnnotations(annotations)

	return newDeployment
}

// WithAnnotations returns a copy of the wrapped appsv1.Deployment object with
// the annotations set to the given value.
func (d *TestDeployment) WithAnnotations(a map[string]string) *appsv1.Deployment {
	newDeployment := d.DeploymentCopy()
	newDeployment.SetAnnotations(a)

	return newDeployment
}

// TestClusterOperator wraps the ClusterOperator type to add helper methods.
type TestClusterOperator struct {
	configv1.ClusterOperator
}

// NewTestClusterOperator returns a new TestDeployment wrapping the given
// OpenShift ClusterOperator object.
func NewTestClusterOperator(co *configv1.ClusterOperator) *TestClusterOperator {
	return &TestClusterOperator{ClusterOperator: *co}
}

// ClusterOperatorCopy returns a deep copy of the wrapped object.
func (co *TestClusterOperator) ClusterOperatorCopy() *configv1.ClusterOperator {
	newCO := &configv1.ClusterOperator{}
	co.ClusterOperator.DeepCopyInto(newCO)

	return newCO
}

// WithConditions returns a copy of the wrapped ClusterOperator object with the
// status conditions set to the given list.
func (co *TestClusterOperator) WithConditions(conds []configv1.ClusterOperatorStatusCondition) *configv1.ClusterOperator {
	newCO := co.ClusterOperatorCopy()
	newCO.Status.Conditions = conds

	return newCO
}
