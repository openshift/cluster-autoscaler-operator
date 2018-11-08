package clusterautoscaler

import (
	"testing"

	autoscalingv1alpha1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	NvidiaGPU     = "nvidia.com/gpu"
	TestNamespace = "test"
)

var (
	PodPriorityThreshold int32 = -10
	MaxPodGracePeriod    int32 = 60
	MaxNodesTotal        int32 = 100
	CoresMin             int32 = 16
	CoresMax             int32 = 32
	MemoryMin            int32 = 32
	MemoryMax            int32 = 64
	NvidiaGPUMin         int32 = 4
	NvidiaGPUMax         int32 = 8
)

func NewClusterAutoscaler() *autoscalingv1alpha1.ClusterAutoscaler {
	// TODO: Maybe just deserialize this from a YAML file?
	return &autoscalingv1alpha1.ClusterAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterAutoscaler",
			APIVersion: "autoscaling.openshift.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: TestNamespace,
		},
		Spec: autoscalingv1alpha1.ClusterAutoscalerSpec{
			MaxPodGracePeriod:    &MaxPodGracePeriod,
			PodPriorityThreshold: &PodPriorityThreshold,
			ResourceLimits: &autoscalingv1alpha1.ResourceLimits{
				MaxNodesTotal: &MaxNodesTotal,
				Cores: &autoscalingv1alpha1.ResourceRange{
					Min: CoresMin,
					Max: CoresMax,
				},
				Memory: &autoscalingv1alpha1.ResourceRange{
					Min: MemoryMin,
					Max: MemoryMax,
				},
				GPUS: []autoscalingv1alpha1.GPULimit{
					{
						Type: NvidiaGPU,
						ResourceRange: autoscalingv1alpha1.ResourceRange{
							Min: NvidiaGPUMin,
							Max: NvidiaGPUMax,
						},
					},
				},
			},
		},
	}
}

func includeString(list []string, item string) bool {
	for i := range list {
		if list[i] == item {
			return true
		}
	}

	return false
}

func TestAutoscalerArgs(t *testing.T) {
	ca := NewClusterAutoscaler()

	args := AutoscalerArgs(ca, TestNamespace)

	expected := []string{
		ExpendablePodsPriorityCutoffArg.Value(PodPriorityThreshold),
		MaxGracefulTerminationSecArg.Value(MaxPodGracePeriod),
		MaxNodesTotalArg.Value(MaxNodesTotal),
		CoresTotalArg.Range(int(CoresMin), int(CoresMax)),
	}

	for _, e := range expected {
		if !includeString(args, e) {
			t.Fatalf("missing arg: %s", e)
		}
	}
}
