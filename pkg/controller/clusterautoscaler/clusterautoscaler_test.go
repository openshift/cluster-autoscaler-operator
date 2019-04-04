package clusterautoscaler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1alpha1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1alpha1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/cluster-autoscaler-operator/test/helpers"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NvidiaGPU          = "nvidia.com/gpu"
	TestNamespace      = "test-namespace"
	TestCloudProvider  = "testProvider"
	TestReleaseVersion = "v100"
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

var TestReconcilerConfig = &Config{
	Name:           "test",
	Namespace:      TestNamespace,
	CloudProvider:  TestCloudProvider,
	ReleaseVersion: TestReleaseVersion,
	Image:          "test/test:v100",
	Replicas:       10,
	Verbosity:      10,
}

func init() {
	apis.AddToScheme(scheme.Scheme)
}

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
						Min:  NvidiaGPUMin,
						Max:  NvidiaGPUMax,
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

// This test ensures we can actually get an autoscaler with fakeclient/client.
// fakeclient.NewFakeClientWithScheme will os.Exit(1) with invalid scheme.
func TestCanGetca(t *testing.T) {
	_ = fakeclient.NewFakeClient(NewClusterAutoscaler())
}

// newFakeReconciler returns a new reconcile.Reconciler with a fake client
func newFakeReconciler(initObjects ...runtime.Object) *Reconciler {
	fakeClient := fakeclient.NewFakeClient(initObjects...)
	return &Reconciler{
		client:   fakeClient,
		scheme:   scheme.Scheme,
		recorder: record.NewFakeRecorder(128),
		config:   TestReconcilerConfig,
	}
}

// The only time Reconcile() should fail is if there's a problem calling the
// api; that failure mode is not currently captured in this test.
func TestReconcile(t *testing.T) {
	ca := NewClusterAutoscaler()
	dep1 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-autoscaler-test",
			Namespace: TestNamespace,
			Annotations: map[string]string{
				util.ReleaseVersionAnnotation: "test-1",
			},
			Generation: 1,
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			Replicas:           1,
			AvailableReplicas:  1,
		},
	}
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: TestNamespace,
			Name:      "test",
		},
	}
	cfg1 := Config{
		ReleaseVersion: "test-1",
		Name:           "test",
		Namespace:      TestNamespace,
	}
	cfg2 := Config{
		ReleaseVersion: "test-1",
		Name:           "test2",
		Namespace:      TestNamespace,
	}
	tCases := []struct {
		expectedError error
		expectedRes   reconcile.Result
		c             *Config
		d             *appsv1.Deployment
	}{
		// Case 0: should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             &cfg1,
			d:             &dep1,
		},
		// Case 1: no ca found, should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             &cfg2,
			d:             &dep1,
		},
		// Case 2: no dep found, should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             &cfg1,
			d:             &appsv1.Deployment{},
		},
	}
	for i, tc := range tCases {
		r := newFakeReconciler(ca, tc.d)
		r.SetConfig(tc.c)
		res, err := r.Reconcile(req)
		assert.Equal(t, tc.expectedRes, res, "case %v: expected res incorrect", i)
		assert.Equal(t, tc.expectedError, err, "case %v: expected err incorrect", i)
	}
}

func TestObjectReference(t *testing.T) {
	testCases := []struct {
		label     string
		object    runtime.Object
		reference *corev1.ObjectReference
	}{
		{
			label: "no namespace",
			object: &autoscalingv1alpha1.ClusterAutoscaler{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterAutoscaler",
					APIVersion: "autoscaling.openshift.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-scoped",
				},
			},
			reference: &corev1.ObjectReference{
				Kind:       "ClusterAutoscaler",
				APIVersion: "autoscaling.openshift.io/v1alpha1",
				Name:       "cluster-scoped",
				Namespace:  TestNamespace,
			},
		},
		{
			label: "existing namespace",
			object: &autoscalingv1alpha1.ClusterAutoscaler{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterAutoscaler",
					APIVersion: "autoscaling.openshift.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-scoped",
					Namespace: "should-not-change",
				},
			},
			reference: &corev1.ObjectReference{
				Kind:       "ClusterAutoscaler",
				APIVersion: "autoscaling.openshift.io/v1alpha1",
				Name:       "cluster-scoped",
				Namespace:  "should-not-change",
			},
		},
	}

	r := newFakeReconciler()

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			ref := r.objectReference(tc.object)
			if ref == nil {
				t.Error("could not create object reference")
			}

			if !equality.Semantic.DeepEqual(tc.reference, ref) {
				t.Errorf("got %v, want %v", ref, tc.reference)
			}
		})
	}
}

func TestUpdateAnnotations(t *testing.T) {
	deployment := helpers.NewTestDeployment(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
		},
	})

	expected := map[string]string{
		util.CriticalPodAnnotation:    "",
		util.ReleaseVersionAnnotation: TestReleaseVersion,
	}

	testCases := []struct {
		label  string
		object metav1.Object
	}{
		{
			label:  "no prior annotations",
			object: deployment.Object(),
		},
		{
			label: "missing version annotation",
			object: deployment.WithAnnotations(map[string]string{
				util.CriticalPodAnnotation: "",
			}).Object(),
		},
		{
			label: "missing critical-pod annotation",
			object: deployment.WithAnnotations(map[string]string{
				util.ReleaseVersionAnnotation: TestReleaseVersion,
			}).Object(),
		},
		{
			label: "old version annotation",
			object: deployment.WithAnnotations(map[string]string{
				util.ReleaseVersionAnnotation: "vOLD",
			}).Object(),
		},
	}

	r := newFakeReconciler()

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			r.UpdateAnnotations(tc.object)

			got := tc.object.GetAnnotations()
			if !equality.Semantic.DeepEqual(got, expected) {
				t.Errorf("got %v, want %v", got, expected)
			}
		})
	}
}
