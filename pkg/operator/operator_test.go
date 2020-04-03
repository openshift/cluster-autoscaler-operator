package operator

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
)

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
			Resource: "namespaces",
			Name:     DefaultWatchNamespace,
		},
	}

	operator := &Operator{config: NewConfig()}
	got := operator.RelatedObjects()
	if !equality.Semantic.DeepEqual(got, expected) {
		t.Errorf("expected %+v, got: %+v", expected, got)
	}
}
