package operator

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	// Register autoscaling types with the scheme
	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	// Register config types with the scheme
	if err := configv1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	// Register PartialObjectMetadata types
	metav1.AddMetaToScheme(scheme.Scheme)
}

// fakeManager implements a minimal manager.Manager interface for testing
// It only needs to provide GetClient() for the RelatedObjects() function
type fakeManager struct {
	manager.Manager
	client client.Client
}

func (f *fakeManager) GetClient() client.Client {
	return f.client
}

func TestRelatedObjects(t *testing.T) {
	expected := []configv1.ObjectReference{
		{
			Group:     v1beta1.SchemeGroupVersion.Group,
			Resource:  "machineautoscalers",
			Name:      "",
			Namespace: DefaultWatchNamespace,
		},
		{
			Group:     v1beta1.SchemeGroupVersion.Group,
			Resource:  "clusterautoscalers",
			Name:      "",
			Namespace: DefaultWatchNamespace,
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterroles",
			Name:     "cluster-autoscaler-operator",
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterroles",
			Name:     "cluster-autoscaler",
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterroles",
			Name:     "cluster-autoscaler-operator:cluster-reader",
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterrolebindings",
			Name:     "cluster-autoscaler-operator",
		},
		{
			Group:    "rbac.authorization.k8s.io",
			Resource: "clusterrolebindings",
			Name:     "cluster-autoscaler",
		},
		{
			Resource: "namespaces",
			Name:     DefaultWatchNamespace,
		},
	}

	// Create fake client with no objects (empty cluster)
	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	// Create fake manager
	fakeManager := &fakeManager{
		client: fakeClient,
	}

	operator := &Operator{
		config:  NewConfig(),
		manager: fakeManager,
	}
	got := operator.RelatedObjects()
	if !equality.Semantic.DeepEqual(got, expected) {
		t.Errorf("expected %+v, got: %+v", expected, got)
	}
}

func TestRelatedObjectsWithAutoscaledMachineSetsAndMachines(t *testing.T) {
	// Create test MachineAutoscaler targeting a MachineSet
	machineAutoscaler := &v1beta1.MachineAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ma-1",
			Namespace: DefaultWatchNamespace,
		},
		Spec: v1beta1.MachineAutoscalerSpec{
			MinReplicas: 1,
			MaxReplicas: 10,
			ScaleTargetRef: v1beta1.CrossVersionObjectReference{
				APIVersion: "machine.openshift.io/v1beta1",
				Kind:       "MachineSet",
				Name:       "test-machineset-1",
			},
		},
	}

	// Create test Machines - one owned by autoscaled MachineSet, one not
	machineOwnedByAutoscaledMS := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "machine.openshift.io/v1beta1",
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-1",
			Namespace: DefaultWatchNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "MachineSet",
					Name:       "test-machineset-1", // Owned by autoscaled MachineSet
				},
			},
		},
	}

	machineOwnedByNonAutoscaledMS := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "machine.openshift.io/v1beta1",
			Kind:       "Machine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-2",
			Namespace: DefaultWatchNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "machine.openshift.io/v1beta1",
					Kind:       "MachineSet",
					Name:       "other-machineset", // NOT autoscaled
				},
			},
		},
	}

	// Build list of all runtime objects for fake client
	initObjects := []runtime.Object{
		machineAutoscaler,
		machineOwnedByAutoscaledMS,
		machineOwnedByNonAutoscaledMS,
	}

	// Create fake client with scheme that includes all necessary types
	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRuntimeObjects(initObjects...).
		Build()

	// Create fake manager
	fakeManager := &fakeManager{
		client: fakeClient,
	}

	// Create operator with test config
	operator := &Operator{
		config:  NewConfig(),
		manager: fakeManager,
	}

	// Call RelatedObjects
	relatedObjects := operator.RelatedObjects()

	// Verify autoscaled MachineSet is present
	foundMachineSets := make(map[string]bool)
	for _, obj := range relatedObjects {
		if obj.Group == "machine.openshift.io" && obj.Resource == "machinesets" {
			foundMachineSets[obj.Name] = true
		}
	}

	if !foundMachineSets["test-machineset-1"] {
		t.Error("expected autoscaled MachineSet 'test-machineset-1' not found in RelatedObjects")
	}

	if len(foundMachineSets) != 1 {
		t.Errorf("expected 1 MachineSet, got %d: %v", len(foundMachineSets), foundMachineSets)
	}

	// Verify only the Machine owned by autoscaled MachineSet is present
	foundMachines := make(map[string]bool)
	for _, obj := range relatedObjects {
		if obj.Group == "machine.openshift.io" && obj.Resource == "machines" {
			foundMachines[obj.Name] = true
		}
	}

	if !foundMachines["test-machine-1"] {
		t.Error("expected Machine 'test-machine-1' owned by autoscaled MachineSet not found in RelatedObjects")
	}

	if foundMachines["test-machine-2"] {
		t.Error("Machine 'test-machine-2' owned by non-autoscaled MachineSet should NOT be in RelatedObjects")
	}

	if len(foundMachines) != 1 {
		t.Errorf("expected 1 Machine, got %d: %v", len(foundMachines), foundMachines)
	}

	// Verify static resources are still present
	hasClusterAutoscalers := false
	hasMachineAutoscalers := false
	for _, obj := range relatedObjects {
		if obj.Resource == "clusterautoscalers" {
			hasClusterAutoscalers = true
		}
		if obj.Resource == "machineautoscalers" {
			hasMachineAutoscalers = true
		}
	}

	if !hasClusterAutoscalers {
		t.Error("RelatedObjects missing clusterautoscalers")
	}
	if !hasMachineAutoscalers {
		t.Error("RelatedObjects missing machineautoscalers")
	}
}
