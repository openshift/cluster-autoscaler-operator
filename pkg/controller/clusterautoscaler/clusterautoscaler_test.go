package clusterautoscaler

import (
	"errors"
	"fmt"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1alpha1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"strings"
	"testing"
)

const (
	NvidiaGPU         = "nvidia.com/gpu"
	TestNamespace     = "test"
	TestCloudProvider = "testProvider"
)

var (
	ScaleDownUnneededTime        = "10s"
	ScaleDownDelayAfterAdd       = "60s"
	PodPriorityThreshold   int32 = -10
	MaxPodGracePeriod      int32 = 60
	MaxNodesTotal          int32 = 100
	CoresMin               int32 = 16
	CoresMax               int32 = 32
	MemoryMin              int32 = 32
	MemoryMax              int32 = 64
	NvidiaGPUMin           int32 = 4
	NvidiaGPUMax           int32 = 8
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
			ScaleDown: &autoscalingv1alpha1.ScaleDownConfig{
				Enabled:       true,
				DelayAfterAdd: &ScaleDownDelayAfterAdd,
				UnneededTime:  &ScaleDownUnneededTime,
			},
		},
	}
}

func includesStringWithPrefix(list []string, prefix string) bool {
	for i := range list {
		if strings.HasPrefix(list[i], prefix) {
			return true
		}
	}

	return false
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

	args := AutoscalerArgs(ca, &Config{CloudProvider: TestCloudProvider, Namespace: TestNamespace})

	expected := []string{
		fmt.Sprintf("--scale-down-delay-after-add=%s", ScaleDownDelayAfterAdd),
		fmt.Sprintf("--scale-down-unneeded-time=%s", ScaleDownUnneededTime),
		fmt.Sprintf("--expendable-pods-priority-cutoff=%d", PodPriorityThreshold),
		fmt.Sprintf("--max-graceful-termination-sec=%d", MaxPodGracePeriod),
		fmt.Sprintf("--cores-total=%d:%d", CoresMin, CoresMax),
		fmt.Sprintf("--max-nodes-total=%d", MaxNodesTotal),
		fmt.Sprintf("--namespace=%s", TestNamespace),
		fmt.Sprintf("--cloud-provider=%s", TestCloudProvider),
	}

	for _, e := range expected {
		if !includeString(args, e) {
			t.Fatalf("missing arg: %s", e)
		}
	}

	expectedMissing := []string{
		"--scale-down-delay-after-delete",
		"--scale-down-delay-after-failure",
	}

	for _, e := range expectedMissing {
		if includesStringWithPrefix(args, e) {
			t.Fatalf("found arg expected to be missing: %s", e)
		}
	}
}

type MockReconciler struct {
	getAutoscalerOk bool
	gASerrType      error
	configVersion   string
	isDCCResult     bool
	calledIsDCC     bool
}

func (r *MockReconciler) GetAutoscaler(_ *autoscalingv1alpha1.ClusterAutoscaler) (*appsv1.Deployment, error) {
	dep := &appsv1.Deployment{}

	if !r.getAutoscalerOk {
		return nil, r.gASerrType
	}
	dep.ObjectMeta.Annotations = make(map[string]string)
	dep.ObjectMeta.Annotations["release.openshift.io/version"] = r.configVersion
	return dep, nil
}
func (r *MockReconciler) isDeploymentControllerCurrent(dep *appsv1.Deployment) (bool, error) {
	r.calledIsDCC = true
	return r.isDCCResult, nil
}

// This test ensures we can actually get an autoscaler with fakeclient/client.
// fakeclient.NewFakeClientWithScheme will os.Exit(1) with invalid scheme.
func TestCanGetca(t *testing.T) {
	tscheme := runtime.NewScheme()
	apis.AddToScheme(tscheme)
	_ = fakeclient.NewFakeClientWithScheme(tscheme, NewClusterAutoscaler())
}

func TestIsDeploymentUpdated(t *testing.T) {
	deploymentError := errors.New("standard error")
	s := schema.GroupResource{Group: "", Resource: "testing"}
	notFoundError := kerrors.NewNotFound(s, "0")
	tCases := []struct {
		expectedError     error
		expectedOk        bool
		expectedCalledDCC bool
		c                 *Config
		r                 *MockReconciler
	}{
		// Case 0:  Everything should work, return true, nil
		{
			expectedError:     nil,
			expectedOk:        true,
			expectedCalledDCC: true,
			c: &Config{
				ReleaseVersion: "test-1",
			},
			r: &MockReconciler{
				getAutoscalerOk: true,
				configVersion:   "test-1",
				isDCCResult:     true,
				gASerrType:      nil,
			},
		},
		// Case 1: Waiting on deployment; should return false, nil.
		{
			expectedError:     nil,
			expectedOk:        false,
			expectedCalledDCC: false,
			c: &Config{
				ReleaseVersion: "test-1",
			},
			r: &MockReconciler{
				getAutoscalerOk: false,
				configVersion:   "test-1",
				isDCCResult:     true,
				gASerrType:      notFoundError,
			},
		},
		// Case 2: Error getting deployment; should return false, err.
		{
			expectedError:     deploymentError,
			expectedOk:        false,
			expectedCalledDCC: false,
			c: &Config{
				ReleaseVersion: "test-1",
			},
			r: &MockReconciler{
				getAutoscalerOk: false,
				configVersion:   "test-1",
				isDCCResult:     true,
				gASerrType:      deploymentError,
			},
		},
		// Case 3: isDeploymentControllerCurrent returns false; should return false, nil.
		{
			expectedError:     nil,
			expectedOk:        false,
			expectedCalledDCC: true,
			c: &Config{
				ReleaseVersion: "test-1",
			},
			r: &MockReconciler{
				getAutoscalerOk: true,
				configVersion:   "test-1",
				isDCCResult:     false,
				gASerrType:      nil,
			},
		},
	}

	ca := NewClusterAutoscaler()
	for i, tc := range tCases {
		tc.r.calledIsDCC = false
		ok, err := isDeploymentUpdated(tc.r, ca, tc.c)
		assert.Equal(t, tc.expectedError, err, "case %v: incorrect error", i)
		assert.Equal(t, tc.expectedOk, ok, "case %v: incorrect ok", i)
		assert.Equal(t, tc.expectedCalledDCC, tc.r.calledIsDCC, "case %v: incorrect calledIsDCC", i)
	}
}
