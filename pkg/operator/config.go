package operator

const (
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
		ClusterAutoscalerNamespace: DefaultClusterAutoscalerNamespace,
		ClusterAutoscalerName:      DefaultClusterAutoscalerName,
		ClusterAutoscalerImage:     DefaultClusterAutoscalerImage,
		ClusterAutoscalerReplicas:  DefaultClusterAutoscalerReplicas,
	}
}
