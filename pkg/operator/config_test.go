package operator

import (
	"os"
	"reflect"
	"testing"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	if config == nil {
		t.Fatal("got a nil config object")
	}

	if config.ClusterAutoscalerNamespace != DefaultClusterAutoscalerNamespace {
		t.Fatal("missing default for ClusterAutoscalerNamespace")
	}
}

func TestConfigFromEnvironment(t *testing.T) {
	testCase := []struct {
		envVars        map[string]string
		expectedConfig *Config
		expectedError  bool
	}{
		{
			envVars: map[string]string{
				"WEBHOOKS_PORT":                "1234",
				"METRICS_PORT":                 "5678",
				"LEADER_ELECTION":              "false",
				"CLUSTER_AUTOSCALER_VERBOSITY": "5",
				"WEBHOOKS_ENABLED":             "false",
			},
			expectedConfig: &Config{
				WatchNamespace:                 DefaultWatchNamespace,
				LeaderElection:                 false,
				LeaderElectionNamespace:        DefaultLeaderElectionNamespace,
				LeaderElectionID:               DefaultLeaderElectionID,
				ClusterAutoscalerNamespace:     DefaultClusterAutoscalerNamespace,
				ClusterAutoscalerName:          DefaultClusterAutoscalerName,
				ClusterAutoscalerImage:         DefaultClusterAutoscalerImage,
				ClusterAutoscalerReplicas:      DefaultClusterAutoscalerReplicas,
				ClusterAutoscalerCloudProvider: DefaultClusterAutoscalerCloudProvider,
				ClusterAutoscalerVerbosity:     5,
				WebhooksEnabled:                false,
				WebhooksPort:                   1234,
				WebhooksCertDir:                DefaultWebhooksCertDir,
				MetricsPort:                    5678,
			},
			expectedError: false,
		},
		{
			envVars: map[string]string{
				"METRICS_PORT": "bad_metrics_port",
			},
			expectedConfig: nil,
			expectedError:  true,
		},
		{
			envVars: map[string]string{
				"WEBHOOKS_PORT": "bad_webhook_port",
			},
			expectedConfig: nil,
			expectedError:  true,
		},
		{
			envVars: map[string]string{
				"LEADER_ELECTION": "bad_leader_election",
			},
			expectedConfig: nil,
			expectedError:  true,
		},
		{
			envVars: map[string]string{
				"CLUSTER_AUTOSCALER_VERBOSITY": "bad_verbosity",
			},
			expectedConfig: nil,
			expectedError:  true,
		},
		{
			envVars: map[string]string{
				"WEBHOOKS_ENABLED": "bad_webhooks_enabled",
			},
			expectedConfig: nil,
			expectedError:  true,
		},
	}

	for _, tc := range testCase {
		for key, val := range tc.envVars {
			os.Setenv(key, val)
		}
		got, err := ConfigFromEnvironment()
		if (err != nil) != tc.expectedError {
			t.Errorf("expected %v, got: %v", tc.expectedError, err)
		}
		if !reflect.DeepEqual(got, tc.expectedConfig) {
			t.Errorf("expected: %v, got: %v", tc.expectedConfig, got)
		}
		for key := range tc.envVars {
			os.Unsetenv(key)
		}
	}
}
