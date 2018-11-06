package operator

const (
	// DefaultClusterAutoscalerName is the default ClusterAutoscaler
	// object watched by the operator.
	DefaultClusterAutoscalerName = "default"
)

// Config represents the runtime configuration for the operator.
type Config struct {
	// ClusterAutoscalerName is the name of the ClusterAutoscaler
	// resource that will be watched by the operator.
	ClusterAutoscalerName string
}

// NewConfig returns a new Config object with defaults set.
func NewConfig() *Config {
	return &Config{
		ClusterAutoscalerName: DefaultClusterAutoscalerName,
	}
}
