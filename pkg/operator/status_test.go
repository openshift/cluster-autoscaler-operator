package operator

import (
	configv1 "github.com/openshift/api/config/v1"
	osconfigv1 "github.com/openshift/api/config/v1"
	fakeconfigclientset "github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCheckMachineAPI(t *testing.T) {
	tConditions := []struct {
		expectedErr  error
		expectedBool bool
		conditions   []osconfigv1.ClusterOperatorStatusCondition
	}{
		{
			expectedErr:  nil,
			expectedBool: true,
			conditions: []osconfigv1.ClusterOperatorStatusCondition{
				{
					Type:   configv1.OperatorAvailable,
					Status: osconfigv1.ConditionTrue,
				},
				{
					Type:   configv1.OperatorFailing,
					Status: osconfigv1.ConditionFalse,
				},
			},
		},
		{
			expectedErr:  nil,
			expectedBool: false,
			conditions: []osconfigv1.ClusterOperatorStatusCondition{
				{
					Type:   configv1.OperatorAvailable,
					Status: osconfigv1.ConditionFalse,
				},
				{
					Type:   configv1.OperatorFailing,
					Status: osconfigv1.ConditionFalse,
				},
			},
		},
	}
	co := &osconfigv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: "machine-api"},
		Status:     osconfigv1.ClusterOperatorStatus{},
	}
	for i, tc := range tConditions {
		co.Status.Conditions = tc.conditions
		r := StatusReporter{
			client:         fakeconfigclientset.NewSimpleClientset(co),
			relatedObjects: []configv1.ObjectReference{},
		}
		res, err := r.CheckMachineAPI()
		assert.Equal(t, tc.expectedBool, res, "case %v: return expected %v but didn't get it", i, tc.expectedBool)
		assert.Equal(t, tc.expectedErr, err, "case %v: expected %v error but didn't get it, got: ", i, tc.expectedErr, err)
	}
}

func TestApplyConditions(t *testing.T) {
	conditions := []configv1.ClusterOperatorStatusCondition{
		{
			Type:    configv1.OperatorAvailable,
			Status:  configv1.ConditionTrue,
			Reason:  "testing",
			Message: "testing",
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
	co := &osconfigv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler"},
		Status:     osconfigv1.ClusterOperatorStatus{},
	}
	r := StatusReporter{
		client:         fakeconfigclientset.NewSimpleClientset(co),
		relatedObjects: []configv1.ObjectReference{},
	}
	err := r.ApplyConditions(conditions)
	assert.Equal(t, nil, err, "expected nil error")
	co_check, err2 := r.GetOrCreateClusterOperator()
	assert.Equal(t, nil, err2, "expected nil error2")
	// Need to check a specific field as comparing all conditions time stamps
	// will be off.
	assert.Equal(t, configv1.ConditionTrue, co_check.Status.Conditions[0].Status, "expected same conditions")
}

func TestCompareVersions(t *testing.T) {
	type tCase struct {
		oV           []osconfigv1.OperandVersion
		expectedBool bool
		expectedErr  error
	}
	tCases := []tCase{
		{
			oV: []osconfigv1.OperandVersion{
				{
					Name:    "operator",
					Version: "1.0",
				},
			},
			expectedBool: true,
			expectedErr:  nil,
		},
		{
			oV: []osconfigv1.OperandVersion{
				{
					Name:    "operator",
					Version: "2.0",
				},
			},
			expectedBool: false,
			expectedErr:  nil,
		},
	}
	desiredVersions := []osconfigv1.OperandVersion{
		{
			Name:    "operator",
			Version: "2.0",
		},
	}
	co := &osconfigv1.ClusterOperator{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler"}}
	for i, tc := range tCases {
		co.Status.Versions = tc.oV
		r := StatusReporter{
			client:         fakeconfigclientset.NewSimpleClientset(co),
			relatedObjects: []configv1.ObjectReference{},
		}
		isProgress, err := r.IsDifferentVersions(desiredVersions)
		assert.Equal(t, tc.expectedBool, isProgress, "case %v: return expected %v but didn't get it", i, tc.expectedBool)
		assert.Equal(t, tc.expectedErr, err, "case %v: expected %v error but didn't get it, got: ", i, tc.expectedErr, err)
	}
}

func TestPrintOperandVersions(t *testing.T) {
	desiredVersions := []osconfigv1.OperandVersion{
		{
			Name:    "operator",
			Version: "2.0",
		},
		{
			Name:    "operand",
			Version: "2.0",
		},
	}
	expectedString := "operator: 2.0, operand: 2.0"
	vS := printOperandVersions(desiredVersions)
	assert.Equal(t, expectedString, vS, "Expected: %v but got: %v", expectedString, vS)
}
