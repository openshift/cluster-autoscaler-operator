package operator

const (
	// DefaultClusterAutoscalerName is the default ClusterAutoscaler
	// object watched by the operator.
	DefaultClusterAutoscalerName = "default"

	// DefaultClusterAutoscalerImage is the default image used in
	// ClusterAutoscaler deployments.
	//
	// TODO(bison): This should obviously be moved to the official
	// namespace once cluster-api support is merged in the OpenShift
	// fork.
	DefaultClusterAutoscalerImage = "quay.io/bison/cluster-autoscaler:a554b4f5"

	// DefaultClusterAutoscalerReplicas is the default number of
	// replicas in ClusterAutoscaler deployments.
	DefaultClusterAutoscalerReplicas = 1
)

// Config represents the runtime configuration for the operator.
type Config struct {
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
		ClusterAutoscalerName:     DefaultClusterAutoscalerName,
		ClusterAutoscalerImage:    DefaultClusterAutoscalerImage,
		ClusterAutoscalerReplicas: DefaultClusterAutoscalerReplicas,
	}
}
