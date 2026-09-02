package machineautoscaler

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createOrUpdateMaCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	updatedConditions := make([]metav1.Condition, len(conditions))
	copy(updatedConditions, conditions)

	for i := range updatedConditions {
		if updatedConditions[i].Reason == newCondition.Reason {
			updatedConditions[i] = newCondition
			return updatedConditions
		}
	}

	return append(updatedConditions, newCondition)
}

func createCondition(status metav1.ConditionStatus, condType, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}
