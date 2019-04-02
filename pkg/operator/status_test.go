package operator

import (
	"fmt"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	fakeconfigclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1alpha1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/cluster-autoscaler-operator/test/helpers"
	cvorm "github.com/openshift/cluster-version-operator/lib/resourcemerge"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	apis.AddToScheme(scheme.Scheme)
}

var ClusterOperatorGroupResource = schema.ParseGroupResource("clusteroperators.config.openshift.io")

var ErrMachineAPINotFound = errors.NewNotFound(ClusterOperatorGroupResource, "machine-api")

var (
	// Available is the list of expected conditions for the operator
	// when reporting as available and updated.
	AvailableConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:   configv1.OperatorAvailable,
			Status: configv1.ConditionTrue,
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

	// FailingConditions is the list of expected conditions for the operator
	// when reporting as failing.
	FailingConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:   configv1.OperatorAvailable,
			Status: configv1.ConditionTrue,
		},
		{
			Type:   configv1.OperatorProgressing,
			Status: configv1.ConditionFalse,
		},
		{
			Type:   configv1.OperatorFailing,
			Status: configv1.ConditionTrue,
		},
	}

	// ProgressingConditions is the list of expected conditions for the operator
	// when reporting as progressing.
	ProgressingConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:   configv1.OperatorAvailable,
			Status: configv1.ConditionTrue,
		},
		{
			Type:   configv1.OperatorProgressing,
			Status: configv1.ConditionTrue,
		},
		{
			Type:   configv1.OperatorFailing,
			Status: configv1.ConditionFalse,
		},
	}
)

const (
	ClusterAutoscalerName      = "test"
	ClusterAutoscalerNamespace = "test-namespace"
	ReleaseVersion             = "v100.0.1"
)

var TestStatusReporterConfig = StatusReporterConfig{
	ClusterAutoscalerName:      ClusterAutoscalerName,
	ClusterAutoscalerNamespace: ClusterAutoscalerNamespace,
	ReleaseVersion:             ReleaseVersion,
	RelatedObjects:             []configv1.ObjectReference{},
}

// clusterAutoscaler is the default ClusterAutoscaler object used in test setup.
var clusterAutoscaler = &autoscalingv1alpha1.ClusterAutoscaler{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ClusterAutoscaler",
		APIVersion: "autoscaling.openshift.io/v1alpha1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: ClusterAutoscalerName,
	},
}

// machineAPI is a ClusterOperator object representing the status of a mock
// machine-api-operator.
var machineAPI = helpers.NewTestClusterOperator(&configv1.ClusterOperator{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ClusterOperator",
		APIVersion: "config.openshift.io/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "machine-api",
	},
})

// deployment represents the default ClusterAutoscaler deployment object.
var deployment = helpers.NewTestDeployment(&appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      fmt.Sprintf("cluster-autoscaler-%s", ClusterAutoscalerName),
		Namespace: ClusterAutoscalerNamespace,
		Annotations: map[string]string{
			util.ReleaseVersionAnnotation: ReleaseVersion,
		},
	},
	Status: appsv1.DeploymentStatus{
		AvailableReplicas: 1,
		UpdatedReplicas:   1,
		Replicas:          1,
	},
})

func TestCheckMachineAPI(t *testing.T) {
	testCases := []struct {
		label        string
		expectedBool bool
		expectedErr  error
		configObjs   []runtime.Object
	}{
		{
			label:        "machine-api available",
			expectedBool: true,
			expectedErr:  nil,
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions),
			},
		},
		{
			label:        "machine-api failing",
			expectedBool: false,
			expectedErr:  nil,
			configObjs: []runtime.Object{
				machineAPI.WithConditions(FailingConditions),
			},
		},
		{
			label:        "machine-api not found",
			expectedBool: false,
			expectedErr:  ErrMachineAPINotFound,
			configObjs:   []runtime.Object{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			reporter := &StatusReporter{
				client:       fakeclient.NewFakeClient(),
				configClient: fakeconfigclient.NewSimpleClientset(tc.configObjs...),
				config:       &TestStatusReporterConfig,
			}

			ok, err := reporter.CheckMachineAPI()

			if ok != tc.expectedBool {
				t.Errorf("got %t, want %t", ok, tc.expectedBool)
			}

			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Errorf("got %v, want %v", err, tc.expectedErr)
			}
		})
	}
}

func TestCheckCheckClusterAutoscaler(t *testing.T) {
	testCases := []struct {
		label        string
		expectedBool bool
		expectedErr  error
		objects      []runtime.Object
	}{
		{
			label:        "no cluster-autoscaler",
			expectedBool: true,
			expectedErr:  nil,
			objects:      []runtime.Object{},
		},
		{
			label:        "no deployment",
			expectedBool: false,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
			},
		},
		{
			label:        "deployment wrong version",
			expectedBool: false,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
				deployment.WithReleaseVersion("vBAD"),
			},
		},
		{
			label:        "deployment not available",
			expectedBool: false,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
				deployment.WithAvailableReplicas(0),
			},
		},
		{
			label:        "available and updated",
			expectedBool: true,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
				deployment.DeploymentCopy(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			reporter := &StatusReporter{
				client:       fakeclient.NewFakeClient(tc.objects...),
				configClient: fakeconfigclient.NewSimpleClientset(),
				config:       &TestStatusReporterConfig,
			}

			ok, err := reporter.CheckClusterAutoscaler()

			if ok != tc.expectedBool {
				t.Errorf("got %t, want %t", ok, tc.expectedBool)
			}

			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Errorf("got %v, want %v", err, tc.expectedErr)
			}
		})
	}
}

func TestStatusChanges(t *testing.T) {
	testCases := []struct {
		label      string
		expected   []configv1.ClusterOperatorStatusCondition
		transition func(*StatusReporter) error
	}{
		{
			label:    "available",
			expected: AvailableConditions,
			transition: func(r *StatusReporter) error {
				return r.available("AvailableReason", "available message")
			},
		},
		{
			label:    "progressing",
			expected: ProgressingConditions,
			transition: func(r *StatusReporter) error {
				return r.progressing("ProgressingReason", "progressing message")
			},
		},
		{
			label:    "failing",
			expected: FailingConditions,
			transition: func(r *StatusReporter) error {
				return r.failing("FailingReason", "failing message")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			reporter := &StatusReporter{
				client:       fakeclient.NewFakeClient(),
				configClient: fakeconfigclient.NewSimpleClientset(),
				config:       &TestStatusReporterConfig,
			}

			err := tc.transition(reporter)
			if err != nil {
				t.Errorf("error applying status: %v", err)
			}

			co, err := reporter.GetClusterOperator()
			if err != nil {
				t.Errorf("error getting ClusterOperator: %v", err)
			}

			for _, cond := range tc.expected {
				ok := cvorm.IsOperatorStatusConditionPresentAndEqual(
					co.Status.Conditions, cond.Type, cond.Status,
				)

				if !ok {
					t.Errorf("wrong status for condition: %s", cond.Type)
				}
			}
		})
	}
}

func TestReportStatus(t *testing.T) {
	testCases := []struct {
		label         string
		expectedBool  bool
		expectedErr   error
		expectedConds []configv1.ClusterOperatorStatusCondition
		clientObjs    []runtime.Object
		configObjs    []runtime.Object
	}{
		{
			label:         "machine-api not found",
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: FailingConditions,
			clientObjs:    []runtime.Object{},
			configObjs:    []runtime.Object{},
		},
		{
			label:         "machine-api not ready",
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: FailingConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(FailingConditions),
			},
		},
		{
			label:         "no cluster-autoscaler",
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions),
			},
		},
		{
			label:         "no cluster-autoscaler deployment",
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: ProgressingConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions),
			},
		},
		{
			label:         "deployment wrong version",
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: ProgressingConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
				deployment.WithReleaseVersion("vWRONG"),
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions),
			},
		},
		{
			label:         "available and updated",
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
				deployment.WithReleaseVersion(ReleaseVersion),
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			reporter := &StatusReporter{
				client:       fakeclient.NewFakeClient(tc.clientObjs...),
				configClient: fakeconfigclient.NewSimpleClientset(tc.configObjs...),
				config:       &TestStatusReporterConfig,
			}

			ok, err := reporter.ReportStatus()

			if ok != tc.expectedBool {
				t.Errorf("got %t, want %t", ok, tc.expectedBool)
			}

			if !equality.Semantic.DeepEqual(err, tc.expectedErr) {
				t.Errorf("got %v, want %v", err, tc.expectedErr)
			}

			// Check that the ClusterOperator status is updated.
			co, err := reporter.GetClusterOperator()
			if err != nil {
				t.Errorf("error getting ClusterOperator: %v", err)
			}

			for _, cond := range tc.expectedConds {
				ok := cvorm.IsOperatorStatusConditionPresentAndEqual(
					co.Status.Conditions, cond.Type, cond.Status,
				)

				if !ok {
					t.Errorf("wrong status for condition: %s", cond.Type)
				}
			}
		})
	}
}
