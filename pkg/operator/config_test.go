package operator

import "testing"

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config == nil {
		t.Fatal("got a nil config object")
	}

	if config.ClusterAutoscalerNamespace != DefaultClusterAutoscalerNamespace {
		t.Fatal("missing default for ClusterAutoscalerNamespace")
	}
}
