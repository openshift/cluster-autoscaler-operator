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
		assert.Equal(t, res, tc.expectedBool, "case %v: return expected %v but didn't get it", i, tc.expectedBool)
		assert.Equal(t, err, tc.expectedErr, "case %v: expected %v error but didn't get it, got: ", i, tc.expectedErr, err)
	}
}
