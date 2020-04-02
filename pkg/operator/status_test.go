package operator

import (
	"fmt"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	fakeconfigclient "github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/cluster-autoscaler-operator/test/helpers"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
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
	// ConditionTransitionTime is the default LastTransitionTime for
	// ClusterOperatorStatusCondition fixture objects.
	ConditionTransitionTime = metav1.NewTime(time.Date(
		2009, time.November, 10, 23, 0, 0, 0, time.UTC,
	))

	// Available is the list of expected conditions for the operator
	// when reporting as available and updated.
	AvailableConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:               configv1.OperatorAvailable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorProgressing,
			Status:             configv1.ConditionFalse,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorDegraded,
			Status:             configv1.ConditionFalse,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorUpgradeable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
	}

	// DegradedConditions is the list of expected conditions for the operator
	// when reporting as degraded.
	DegradedConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:               configv1.OperatorAvailable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorProgressing,
			Status:             configv1.ConditionFalse,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorDegraded,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorUpgradeable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
	}

	// ProgressingConditions is the list of expected conditions for the operator
	// when reporting as progressing.
	ProgressingConditions = []configv1.ClusterOperatorStatusCondition{
		{
			Type:               configv1.OperatorAvailable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorProgressing,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorDegraded,
			Status:             configv1.ConditionFalse,
			LastTransitionTime: ConditionTransitionTime,
		},
		{
			Type:               configv1.OperatorUpgradeable,
			Status:             configv1.ConditionTrue,
			LastTransitionTime: ConditionTransitionTime,
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
var clusterAutoscaler = &autoscalingv1.ClusterAutoscaler{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ClusterAutoscaler",
		APIVersion: "autoscaling.openshift.io/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: ClusterAutoscalerName,
	},
}

// Common Kubernetes fixture objects.
var (
	machineAPI = helpers.NewTestClusterOperator(&configv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterOperator",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "machine-api",
		},
	})

	clusterAutoscalerOperator = helpers.NewTestClusterOperator(&configv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterOperator",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-autoscaler",
		},
	})

	deployment = helpers.NewTestDeployment(&appsv1.Deployment{
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
)

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
				machineAPI.WithConditions(AvailableConditions).Object(),
			},
		},
		{
			label:        "machine-api degraded",
			expectedBool: false,
			expectedErr:  nil,
			configObjs: []runtime.Object{
				machineAPI.WithConditions(DegradedConditions).Object(),
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
				deployment.WithReleaseVersion("vBAD").Object(),
			},
		},
		{
			label:        "deployment not available",
			expectedBool: false,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
				deployment.WithAvailableReplicas(0).Object(),
			},
		},
		{
			label:        "available and updated",
			expectedBool: true,
			expectedErr:  nil,
			objects: []runtime.Object{
				clusterAutoscaler,
				deployment.Object(),
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
			label:    "degraded",
			expected: DegradedConditions,
			transition: func(r *StatusReporter) error {
				return r.degraded("DegradedReason", "degraded message")
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
				ok := v1helpers.IsStatusConditionPresentAndEqual(
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
		versionChange bool
		expectedBool  bool
		expectedErr   error
		expectedConds []configv1.ClusterOperatorStatusCondition
		clientObjs    []runtime.Object
		configObjs    []runtime.Object
	}{
		{
			label:         "machine-api not found",
			versionChange: true,
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: DegradedConditions,
			clientObjs:    []runtime.Object{},
			configObjs:    []runtime.Object{},
		},
		{
			label:         "machine-api not ready",
			versionChange: true,
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: DegradedConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(DegradedConditions).Object(),
			},
		},
		{
			label:         "no cluster-autoscaler",
			versionChange: true,
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
			},
		},
		{
			label:         "no cluster-autoscaler deployment",
			versionChange: true,
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: ProgressingConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
			},
		},
		{
			label:         "deployment wrong version",
			versionChange: true,
			expectedBool:  false,
			expectedErr:   nil,
			expectedConds: ProgressingConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
				deployment.WithReleaseVersion("vWRONG").Object(),
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
			},
		},
		{
			label:         "available and updated",
			versionChange: true,
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs: []runtime.Object{
				clusterAutoscaler,
				deployment.WithReleaseVersion(ReleaseVersion).Object(),
			},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
			},
		},
		{
			label:         "no version change",
			versionChange: false,
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
				clusterAutoscalerOperator.
					WithConditions(AvailableConditions).
					WithVersion(ReleaseVersion).
					Object(),
			},
		},
		{
			label:         "version change noop",
			versionChange: true,
			expectedBool:  true,
			expectedErr:   nil,
			expectedConds: AvailableConditions,
			clientObjs:    []runtime.Object{},
			configObjs: []runtime.Object{
				machineAPI.WithConditions(AvailableConditions).Object(),
				clusterAutoscalerOperator.
					WithConditions(AvailableConditions).
					WithVersion("vOLD").
					Object(),
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

			// Check that the ClusterOperator status is created.
			co, err := reporter.GetClusterOperator()
			if err != nil {
				t.Errorf("error getting ClusterOperator: %v", err)
			}

			// Check that all conditions have the expected status.
			for _, cond := range tc.expectedConds {
				ok := v1helpers.IsStatusConditionPresentAndEqual(
					co.Status.Conditions, cond.Type, cond.Status,
				)

				if !ok {
					t.Errorf("wrong status for condition: %s", cond.Type)
				}
			}

			// Check the LastTransitionTime of the Progressing condition.
			for _, v := range co.Status.Versions {
				if v.Name != "operator" {
					continue
				}

				p := v1helpers.FindStatusCondition(
					co.Status.Conditions, configv1.OperatorProgressing,
				)

				if p == nil {
					t.Fatal("expected Progressing condition not found")
				}

				switch tc.versionChange {
				case true:
					// If the version changed, the last transition time should
					// be more recent than the original.
					if !ConditionTransitionTime.Before(&p.LastTransitionTime) {
						t.Error("expected Progressing condition transition time update")
					}

				case false:
					// If the version did not change, the last transition time
					// should remain unchanged.
					if !ConditionTransitionTime.Equal(&p.LastTransitionTime) {
						t.Error("unexpected Progressing condition transition time update")
					}

				default:
					panic("back away slowly...")
				}
			}
		})
	}
}
