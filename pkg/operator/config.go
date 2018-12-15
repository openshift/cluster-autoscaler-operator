package operator

import "os"

const (
	// DefaultWatchNamespace is the default namespace the operator
	// will watch for instances of its custom resources.
	DefaultWatchNamespace = "openshift-cluster-api"

	// DefaultClusterAutoscalerNamespace is the default namespace for
	// cluster-autoscaler deployments.
	DefaultClusterAutoscalerNamespace = "openshift-cluster-api"

	// DefaultClusterAutoscalerName is the default ClusterAutoscaler
	// object watched by the operator.
	DefaultClusterAutoscalerName = "default"

	// DefaultClusterAutoscalerImage is the default image used in
	// ClusterAutoscaler deployments.
	DefaultClusterAutoscalerImage = "quay.io/openshift/origin-cluster-autoscaler:v4.0"

	// DefaultClusterAutoscalerReplicas is the default number of
	// replicas in ClusterAutoscaler deployments.
	DefaultClusterAutoscalerReplicas = 1
)

// Config represents the runtime configuration for the operator.
type Config struct {
	// WatchNamespace is the namespace the operator will watch for
	// ClusterAutoscaler and MachineAutoscaler instances.
	WatchNamespace string

	// ClusterAutoscalerNamespace is the namespace in which
	// cluster-autoscaler deployments will be created.
	ClusterAutoscalerNamespace string

	// ClusterAutoscalerName is the name of the ClusterAutoscaler
	// resource that will be watched by the operator.
	ClusterAutoscalerName string

	// ClusterAutoscalerImage is the image to be used in
	// ClusterAutoscaler deployments.
	ClusterAutoscalerImage string

	// ClusterAutoscalerReplicas is the number of replicas to be
	// configured in ClusterAutoscaler deployments.
	ClusterAutoscalerReplicas int32
}

// NewConfig returns a new Config object with defaults set.
func NewConfig() *Config {
	return &Config{
		WatchNamespace:             DefaultWatchNamespace,
		ClusterAutoscalerNamespace: DefaultClusterAutoscalerNamespace,
		ClusterAutoscalerName:      DefaultClusterAutoscalerName,
		ClusterAutoscalerImage:     DefaultClusterAutoscalerImage,
		ClusterAutoscalerReplicas:  DefaultClusterAutoscalerReplicas,
	}
}

// ConfigFromEnvironment returns a new Config object with defaults
// overridden by environment variables when set.
func ConfigFromEnvironment() *Config {
	config := NewConfig()

	if watchNamespace, ok := os.LookupEnv("WATCH_NAMESPACE"); ok {
		config.WatchNamespace = watchNamespace
	}

	if caName, ok := os.LookupEnv("CLUSTER_AUTOSCALER_NAME"); ok {
		config.ClusterAutoscalerName = caName
	}

	if caImage, ok := os.LookupEnv("CLUSTER_AUTOSCALER_IMAGE"); ok {
		config.ClusterAutoscalerImage = caImage
	}

	if caNamespace, ok := os.LookupEnv("CLUSTER_AUTOSCALER_NAMESPACE"); ok {
		config.ClusterAutoscalerNamespace = caNamespace
	}

	return config
}
