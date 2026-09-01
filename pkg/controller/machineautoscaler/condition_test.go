package machineautoscaler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	mockedDate     = time.Date(2026, time.September, 1, 23, 0, 0, 0, time.UTC)
	mockedDatePrev = time.Date(2026, time.September, 1, 22, 59, 59, 59, time.UTC)
)

func TestCreateOrUpdateCondition(t *testing.T) {
	testCases := map[string]struct {
		prevConditions     []metav1.Condition
		expectedConditions []metav1.Condition
		newCondition       metav1.Condition
	}{
		"create condition": {
			prevConditions: []metav1.Condition{},
			expectedConditions: []metav1.Condition{
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerReadyReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDate),
				},
			},
			newCondition: metav1.Condition{
				Type:               machineAutoscalerReadyType,
				Status:             metav1.ConditionTrue,
				Reason:             machineAutoscalerReadyReason,
				Message:            machineAutoscalerReadyMessage,
				LastTransitionTime: metav1.NewTime(mockedDate),
			},
		},
		"if the reason is different, create condition": {
			prevConditions: []metav1.Condition{
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerNATargetScaleRefReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDate),
				},
			},
			expectedConditions: []metav1.Condition{
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerNATargetScaleRefReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDate),
				},
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerReadyReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDate),
				},
			},
			newCondition: metav1.Condition{
				Type:               machineAutoscalerReadyType,
				Status:             metav1.ConditionTrue,
				Reason:             machineAutoscalerReadyReason,
				Message:            machineAutoscalerReadyMessage,
				LastTransitionTime: metav1.NewTime(mockedDate),
			},
		},
		"update condition": {
			prevConditions: []metav1.Condition{
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerReadyReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDatePrev),
				},
			},
			expectedConditions: []metav1.Condition{
				{
					Type:               machineAutoscalerReadyType,
					Status:             metav1.ConditionTrue,
					Reason:             machineAutoscalerReadyReason,
					Message:            machineAutoscalerReadyMessage,
					LastTransitionTime: metav1.NewTime(mockedDate),
				},
			},
			newCondition: metav1.Condition{
				Type:               machineAutoscalerReadyType,
				Status:             metav1.ConditionTrue,
				Reason:             machineAutoscalerReadyReason,
				Message:            machineAutoscalerReadyMessage,
				LastTransitionTime: metav1.NewTime(mockedDate),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := createOrUpdateMaCondition(tc.prevConditions, tc.newCondition)
			assert.ElementsMatch(t, got, tc.expectedConditions)
		})
	}
}
