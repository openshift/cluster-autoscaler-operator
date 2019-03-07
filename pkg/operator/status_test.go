package operator

import (
	"errors"
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

type MockStatusReporter struct {
	isCheckMachineAPI                 bool
	isCheckMachineAPIFail             bool
	progressingCalled                 bool
	isAvailable                       bool
	availableCalled                   bool
	availableReason, availableMessage string
	failReason, failMessage           string
}

type MockCheck struct {
	isFail                bool
	isAvailableAndUpdated bool
}

func (c *MockCheck) AvailableAndUpdated() (bool, error) {
	if c.isFail {
		return false, errors.New("returning isFail")
	}
	return c.isAvailableAndUpdated, nil
}

func (r *MockStatusReporter) CheckMachineAPI() (bool, error) {
	if r.isCheckMachineAPIFail {
		return false, errors.New("returning failure")
	}
	return r.isCheckMachineAPI, nil
}

func (r *MockStatusReporter) Available(reason, message string) error {
	r.availableCalled = true
	r.availableReason = reason
	r.availableMessage = message
	if !r.isAvailable {
		return errors.New("returning failure")
	} else {
		return nil
	}
}

func (r *MockStatusReporter) Progressing() error {
	r.progressingCalled = true
	return nil
}

func (r *MockStatusReporter) Fail(reason, message string) error {
	r.failReason = reason
	r.failMessage = message
	return nil
}

func TestApplyStatus(t *testing.T) {
	tCases := []struct {
		applyOk                 bool
		applyExpectErr          bool
		expectAvailableCalled   bool
		expectProgressingCalled bool
		failReason              string
		c                       *MockCheck
		r                       *MockStatusReporter
	}{
		{
			// Case 0:  Everything succeeds and available is called;
			// should return true, nil.
			applyOk:                 true,
			applyExpectErr:          false,
			expectAvailableCalled:   true,
			expectProgressingCalled: false,
			c: &MockCheck{
				isFail:                false,
				isAvailableAndUpdated: true,
			},
			r: &MockStatusReporter{
				isCheckMachineAPI:     true,
				isCheckMachineAPIFail: false,
				isAvailable:           true,
				availableCalled:       false,
				progressingCalled:     false,
			},
		},
		{
			// Case 1:  check.AvailableAndUpdated() reports fail;
			// should return false, nil.
			applyOk:                 false,
			applyExpectErr:          true,
			expectAvailableCalled:   false,
			expectProgressingCalled: false,
			failReason:              ReasonCheckAutoscaler,
			c: &MockCheck{
				isFail:                true,
				isAvailableAndUpdated: true,
			},
			r: &MockStatusReporter{
				isCheckMachineAPI:     true,
				isCheckMachineAPIFail: false,
				isAvailable:           true,
				availableCalled:       false,
				progressingCalled:     false,
			},
		},
		{
			// Case 2:  check.AvailableAndUpdated() reports false;
			// should call Progressing; return false, nil.
			applyOk:                 false,
			applyExpectErr:          false,
			expectAvailableCalled:   false,
			expectProgressingCalled: true,
			failReason:              "",
			c: &MockCheck{
				isFail:                false,
				isAvailableAndUpdated: false,
			},
			r: &MockStatusReporter{
				isCheckMachineAPI:     true,
				isCheckMachineAPIFail: false,
				isAvailable:           true,
				availableCalled:       false,
				progressingCalled:     false,
			},
		},
		{
			// Case 3:  CheckMachineAPI() reports false;
			// should fail with ReasonMissingDependency; return false, nil.
			applyOk:                 false,
			applyExpectErr:          false,
			expectAvailableCalled:   false,
			expectProgressingCalled: false,
			failReason:              ReasonMissingDependency,
			c: &MockCheck{
				isFail:                false,
				isAvailableAndUpdated: false,
			},
			r: &MockStatusReporter{
				isCheckMachineAPI:     false,
				isCheckMachineAPIFail: false,
				isAvailable:           true,
				availableCalled:       false,
				progressingCalled:     false,
			},
		},
		{
			// Case 4:  CheckMachineAPI() reports error;
			// should fail with ReasonMissingDependency; return false, nil.
			applyOk:                 false,
			applyExpectErr:          true,
			expectAvailableCalled:   false,
			expectProgressingCalled: false,
			failReason:              ReasonMissingDependency,
			c: &MockCheck{
				isFail:                false,
				isAvailableAndUpdated: false,
			},
			r: &MockStatusReporter{
				isCheckMachineAPI:     false,
				isCheckMachineAPIFail: true,
				isAvailable:           true,
				availableCalled:       false,
				progressingCalled:     false,
			},
		},
	}
	for i, tc := range tCases {
		ok, _ := ApplyStatus(tc.r, tc.c)
		if tc.applyExpectErr {
			assert.Equal(t, tc.failReason, tc.r.failReason, "case %v: incorrect error return", i)
		}
		assert.Equal(t, tc.applyOk, ok, "case %v: incorrect ok", i)
		assert.Equal(t, tc.expectAvailableCalled, tc.r.availableCalled, "case %v: available called incorrect", i)
		assert.Equal(t, tc.r.availableReason, "", "case %v: incorrect ok", i)
		assert.Equal(t, tc.expectProgressingCalled, tc.r.progressingCalled, "case %v: incorrect progressingCalled", i)
		if tc.r.isCheckMachineAPIFail {
			assert.Equal(t, "error checking machine-api operator status returning failure", tc.r.failMessage, "case %v: incorrect failure message")
		}
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
		releaseVersion: "testing-1",
	}
	err := r.ApplyConditions(conditions, true)
	assert.Equal(t, nil, err, "expected nil error")
	co_check, err2 := r.GetOrCreateClusterOperator()
	assert.Equal(t, nil, err2, "expected nil error2")
	// Need to check a specific field as comparing all conditions time stamps
	// will be off.
	assert.Equal(t, configv1.ConditionTrue, co_check.Status.Conditions[0].Status, "expected same conditions")
	assert.Equal(t, "testing-1", co_check.Status.Versions[0].Version, "expected same version")
}
