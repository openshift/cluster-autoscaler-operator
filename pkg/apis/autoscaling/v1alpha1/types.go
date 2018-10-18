package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterAutoscaler `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ClusterAutoscalerSpec   `json:"spec"`
	Status            ClusterAutoscalerStatus `json:"status,omitempty"`
}

// TODO(bison): Which of these should be optional?

type ClusterAutoscalerSpec struct {
	ScanInterval string          `json:"scanInterval"`
	ScaleDown    ScaleDownConfig `json:"scaleDown"`
}

type ClusterAutoscalerStatus struct {
	// Fill me
}

type ScaleDownConfig struct {
	Enabled           bool   `json:"enabled"`
	DelayAfterAdd     string `json:"delayAfterAdd"`
	DelayAfterDelete  string `json:"delayAfterDelete"`
	DelayAfterFailure string `json:"delayAfterFailure"`
}
