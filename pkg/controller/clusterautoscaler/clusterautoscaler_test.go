package clusterautoscaler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/cluster-autoscaler-operator/test/helpers"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NvidiaGPU          = "nvidia.com"
	TestNamespace      = "test-namespace"
	TestCloudProvider  = "testProvider"
	TestReleaseVersion = "v100"
)

var (
	ScaleDownUnneededTime               = "10s"
	ScaleDownUtilizationThreshold       = "0.4"
	ScaleDownDelayAfterAdd              = "60s"
	NewPodScaleUpDelay                  = "10s"
	MaxNodeProvisionTime                = "30m"
	PodPriorityThreshold          int32 = -10
	MaxPodGracePeriod             int32 = 60
	MaxNodesTotal                 int32 = 100
	CoresMin                      int32 = 16
	CoresMax                      int32 = 32
	MemoryMin                     int32 = 32
	MemoryMax                     int32 = 64
	NvidiaGPUMin                  int32 = 4
	NvidiaGPUMax                  int32 = 8
)

var TestReconcilerConfig = Config{
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
	monitoringv1.AddToScheme(scheme.Scheme)
	configv1.AddToScheme(scheme.Scheme)
}

func NewClusterAutoscaler() *autoscalingv1.ClusterAutoscaler {
	// TODO: Maybe just deserialize this from a YAML file?
	return &autoscalingv1.ClusterAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterAutoscaler",
			APIVersion: "autoscaling.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: TestNamespace,
		},
		Spec: autoscalingv1.ClusterAutoscalerSpec{
			MaxPodGracePeriod:    &MaxPodGracePeriod,
			PodPriorityThreshold: &PodPriorityThreshold,
			ResourceLimits: &autoscalingv1.ResourceLimits{
				MaxNodesTotal: &MaxNodesTotal,
				Cores: &autoscalingv1.ResourceRange{
					Min: CoresMin,
					Max: CoresMax,
				},
				Memory: &autoscalingv1.ResourceRange{
					Min: MemoryMin,
					Max: MemoryMax,
				},
				GPUS: []autoscalingv1.GPULimit{
					{
						Type: NvidiaGPU,
						Min:  NvidiaGPUMin,
						Max:  NvidiaGPUMax,
					},
				},
			},
			ScaleDown: &autoscalingv1.ScaleDownConfig{
				Enabled:              true,
				DelayAfterAdd:        &ScaleDownDelayAfterAdd,
				UnneededTime:         &ScaleDownUnneededTime,
				UtilizationThreshold: &ScaleDownUtilizationThreshold,
			},
			ScaleUp: &autoscalingv1.ScaleUpConfig{
				NewPodScaleUpDelay: &NewPodScaleUpDelay,
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

// TestAutoscalerArgsFromSpec validates that command line flags to the
// autoscaler appear in the proper format when set in the ClusterAutoscaler.spec
func TestAutoscalerArgsFromSpec(t *testing.T) {
	testCases := []struct {
		name            string
		caFunc          func() *autoscalingv1.ClusterAutoscaler
		expected        []string
		expectedMissing []string
	}{
		{
			name:   "all default arguments",
			caFunc: NewClusterAutoscaler,
			expected: []string{
				"--logtostderr",
				"--record-duplicated-events",
				"--v=0",
				// TODO elmiko, enable this once ProvisioningRequest is not behind a feature gate
				// "--enable-provisioning-requests=true",
				fmt.Sprintf("--cores-total=%d:%d", CoresMin, CoresMax),
				fmt.Sprintf("--cloud-provider=%s", TestCloudProvider),
				fmt.Sprintf("--expendable-pods-priority-cutoff=%d", PodPriorityThreshold),
				fmt.Sprintf("--leader-elect-lease-duration=%s", leaderElectLeaseDuration),
				fmt.Sprintf("--leader-elect-renew-deadline=%s", leaderElectRenewDeadline),
				fmt.Sprintf("--leader-elect-retry-period=%s", leaderElectRetryPeriod),
				fmt.Sprintf("--max-graceful-termination-sec=%d", MaxPodGracePeriod),
				fmt.Sprintf("--max-nodes-total=%d", MaxNodesTotal),
				fmt.Sprintf("--namespace=%s", TestNamespace),
				fmt.Sprintf("--scale-down-delay-after-add=%s", ScaleDownDelayAfterAdd),
				fmt.Sprintf("--scale-down-unneeded-time=%s", ScaleDownUnneededTime),
				fmt.Sprintf("--scale-down-utilization-threshold=%s", ScaleDownUtilizationThreshold),
				fmt.Sprintf("--new-pod-scale-up-delay=%s", NewPodScaleUpDelay),
			},
			expectedMissing: []string{
				"--scale-down-delay-after-delete",
				"--scale-down-delay-after-failure",
				"--max-node-provision-time",
				"--balance-similar-node-groups",
				"--ignore-daemonsets-utilization",
				"--skip-nodes-with-local-storage",
				"--balancing-ignore-label",
			},
		},
		{
			name: "set boolean options as true",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				ca.Spec.BalanceSimilarNodeGroups = ptr.To(true)
				ca.Spec.IgnoreDaemonsetsUtilization = ptr.To(true)
				ca.Spec.SkipNodesWithLocalStorage = ptr.To(true)
				return ca
			},
			expected: []string{
				fmt.Sprintf("--balance-similar-node-groups=true"),
				fmt.Sprintf("--ignore-daemonsets-utilization=true"),
				fmt.Sprintf("--skip-nodes-with-local-storage=true"),
			},
		},
		{
			name: "set boolean options as false",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				ca.Spec.BalanceSimilarNodeGroups = ptr.To(false)
				ca.Spec.IgnoreDaemonsetsUtilization = ptr.To(false)
				ca.Spec.SkipNodesWithLocalStorage = ptr.To(false)
				return ca
			},
			expected: []string{
				fmt.Sprintf("--balance-similar-node-groups=false"),
				fmt.Sprintf("--ignore-daemonsets-utilization=false"),
				fmt.Sprintf("--skip-nodes-with-local-storage=false"),
			},
		},
		{
			name: "set MaxNodeProvisionTime",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				ca.Spec.MaxNodeProvisionTime = MaxNodeProvisionTime
				return ca
			},
			expected: []string{
				fmt.Sprintf("--max-node-provision-time=%s", MaxNodeProvisionTime),
			},
		},
		{
			name: "set BalancingIgnoredLabels",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				ca.Spec.BalanceSimilarNodeGroups = ptr.To(true)
				ca.Spec.BalancingIgnoredLabels = []string{"test/ignoredLabel", "test/anotherIgnoredLabel"}
				return ca
			},
			expected: []string{
				fmt.Sprintf("--balance-similar-node-groups=true"),
				fmt.Sprintf("--balancing-ignore-label=test/ignoredLabel"),
				fmt.Sprintf("--balancing-ignore-label=test/anotherIgnoredLabel"),
			},
		},
		{
			name: "set Expanders",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				ca.Spec.Expanders = []autoscalingv1.ExpanderString{
					autoscalingv1.PriorityExpander,
					autoscalingv1.LeastWasteExpander,
					autoscalingv1.RandomExpander,
				}
				return ca
			},
			expected: []string{
				fmt.Sprintf("--expander=priority,least-waste,random"),
			},
		},
		{
			name: "set CordonNodeBeforeTerminating to Enabled",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				mode := autoscalingv1.CordonNodeBeforeTerminatingModeEnabled
				ca.Spec.ScaleDown.CordonNodeBeforeTerminating = &mode
				return ca
			},
			expected: []string{
				"--cordon-node-before-terminating=true",
			},
		},
		{
			name: "set CordonNodeBeforeTerminating to Disabled",
			caFunc: func() *autoscalingv1.ClusterAutoscaler {
				ca := NewClusterAutoscaler()
				mode := autoscalingv1.CordonNodeBeforeTerminatingModeDisabled
				ca.Spec.ScaleDown.CordonNodeBeforeTerminating = &mode
				return ca
			},
			expected: []string{
				"--cordon-node-before-terminating=false",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			ca := tc.caFunc()
			args := AutoscalerArgs(ca, &Config{CloudProvider: TestCloudProvider, Namespace: TestNamespace})

			for _, e := range tc.expected {
				if !includeString(args, e) {
					t.Fatalf("missing expected argument: \"%s\"", e)
				}
			}

			for _, e := range tc.expectedMissing {
				if includesStringWithPrefix(args, e) {
					t.Fatalf("found argument expected to be missing: \"%s\"", e)
				}
			}
		})
	}
}

// This test ensures we can actually get an autoscaler with fakeclient/client.
// fakeclient.NewFakeClientWithScheme will os.Exit(1) with invalid scheme.
func TestCanGetca(t *testing.T) {
	_ = fakeclient.NewFakeClient(NewClusterAutoscaler())
}

// newFakeReconciler returns a new reconcile.Reconciler with a fake client
func newFakeReconciler(initObjects ...runtime.Object) *Reconciler {
	fakeClient := fakeclient.
		NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRuntimeObjects(initObjects...).
		WithStatusSubresource(&autoscalingv1.ClusterAutoscaler{}).
		Build()
	return &Reconciler{
		client:    fakeClient,
		scheme:    scheme.Scheme,
		recorder:  record.NewFakeRecorder(128),
		config:    TestReconcilerConfig,
		validator: NewValidator(TestReconcilerConfig.Name, fakeClient, scheme.Scheme),
	}
}

// The only time Reconcile() should fail is if there's a problem calling the
// api; that failure mode is not currently captured in this test.
func TestReconcile(t *testing.T) {
	ca := NewClusterAutoscaler()
	ca.ObjectMeta.Name = "cluster"
	infrastructure := &configv1.Infrastructure{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Infrastructure",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-infrastructure",
			Namespace: TestNamespace,
		},
		Status: configv1.InfrastructureStatus{
			PlatformStatus: &configv1.PlatformStatus{
				Type: configv1.AWSPlatformType,
			},
		},
	}
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
		c             Config
		d             *appsv1.Deployment
	}{
		// Case 0: should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             cfg1,
			d:             &dep1,
		},
		// Case 1: no ca found, should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             cfg2,
			d:             &dep1,
		},
		// Case 2: no dep found, should pass, returns {}, nil.
		{
			expectedError: nil,
			expectedRes:   reconcile.Result{},
			c:             cfg1,
			d:             &appsv1.Deployment{},
		},
	}
	for i, tc := range tCases {
		r := newFakeReconciler(ca, tc.d, infrastructure)
		r.SetConfig(tc.c)
		res, err := r.Reconcile(context.TODO(), req)
		assert.Equal(t, tc.expectedRes, res, "case %v: expected res incorrect", i)
		assert.Equal(t, tc.expectedError, err, "case %v: expected err incorrect", i)
	}
}

func TestCADeleting(t *testing.T) {
	ca := NewClusterAutoscaler()
	now := metav1.Now()
	ca.DeletionTimestamp = &now
	// the fake client requires a finalizer when creating an object with a deletion timestamp
	ca.SetFinalizers([]string{"fake-test-finalizer"})

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: TestNamespace,
			Name:      "test",
		},
	}
	cfg := Config{
		ReleaseVersion: "test-1",
		Name:           "test",
		Namespace:      TestNamespace,
	}

	dep1 := &appsv1.Deployment{
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

	depKey := client.ObjectKey{
		Namespace: dep1.Namespace,
		Name:      dep1.Name,
	}

	testCases := []struct {
		name               string
		existingDeployment *appsv1.Deployment
	}{
		{
			name:               "With no existing deployment",
			existingDeployment: &appsv1.Deployment{},
		},
		{
			name:               "With an existing deployment",
			existingDeployment: dep1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFakeReconciler(ca, tc.existingDeployment)
			r.SetConfig(cfg)
			res, err := r.Reconcile(context.TODO(), req)
			assert.NoError(t, err)
			assert.Equal(t, res, reconcile.Result{})

			// Ensure that after the reconcile, no deployment exists
			err = r.client.Get(context.Background(), depKey, &appsv1.Deployment{})
			assert.Equal(t, err, apierrors.NewNotFound(schema.GroupResource{
				Group:    "apps",
				Resource: "deployments",
			}, dep1.Name))
		})
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
			object: &autoscalingv1.ClusterAutoscaler{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterAutoscaler",
					APIVersion: "autoscaling.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-scoped",
				},
			},
			reference: &corev1.ObjectReference{
				Kind:       "ClusterAutoscaler",
				APIVersion: "autoscaling.openshift.io/v1",
				Name:       "cluster-scoped",
				Namespace:  TestNamespace,
			},
		},
		{
			label: "existing namespace",
			object: &autoscalingv1.ClusterAutoscaler{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterAutoscaler",
					APIVersion: "autoscaling.openshift.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-scoped",
					Namespace: "should-not-change",
				},
			},
			reference: &corev1.ObjectReference{
				Kind:       "ClusterAutoscaler",
				APIVersion: "autoscaling.openshift.io/v1",
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
