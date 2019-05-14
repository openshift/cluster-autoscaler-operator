package operator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/appscode/jsonpatch"
	configv1 "github.com/openshift/api/config/v1"

	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller/clusterautoscaler"
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller/machineautoscaler"
	"k8s.io/apimachinery/pkg/types"
	admissionregistrationv1beta1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// OperatorName is the name of this operator.
const OperatorName = "cluster-autoscaler"

// Operator represents an instance of the cluster-autoscaler-operator.
type Operator struct {
	config  *Config
	manager manager.Manager
}

// New returns a new Operator instance with the given config and a
// manager configured with the various controllers.
func New(cfg *Config) (*Operator, error) {
	operator := &Operator{config: cfg}

	// Get a config to talk to the apiserver.
	clientConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	// Create the controller-manager.
	managerOptions := manager.Options{
		Namespace:               cfg.WatchNamespace,
		LeaderElection:          cfg.LeaderElection,
		LeaderElectionNamespace: cfg.LeaderElectionNamespace,
		LeaderElectionID:        cfg.LeaderElectionID,
	}

	operator.manager, err = manager.New(clientConfig, managerOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %v", err)
	}

	// Setup Scheme for all resources.
	if err := apis.AddToScheme(operator.manager.GetScheme()); err != nil {
		return nil, fmt.Errorf("failed to register types: %v", err)
	}

	// Setup our controllers and add them to the manager.
	if err := operator.AddControllers(); err != nil {
		return nil, fmt.Errorf("failed to add controllers: %v", err)
	}

	// Setup admission webhooks and add them to the manager.
	if cfg.WebhooksEnabled {
		if err := operator.AddWebhooks(); err != nil {
			return nil, fmt.Errorf("failed to start webhook server: %v", err)
		}
	}

	statusConfig := &StatusReporterConfig{
		ClusterAutoscalerName:      cfg.ClusterAutoscalerName,
		ClusterAutoscalerNamespace: cfg.ClusterAutoscalerNamespace,
		ReleaseVersion:             cfg.ReleaseVersion,
		RelatedObjects:             operator.RelatedObjects(),
	}

	statusReporter, err := NewStatusReporter(operator.manager, statusConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create status reporter: %v", err)
	}

	operator.manager.Add(statusReporter)

	return operator, nil
}

// RelatedObjects returns a list of objects related to the operator and its
// operands.  These are used in the ClusterOperator status.
func (o *Operator) RelatedObjects() []configv1.ObjectReference {
	relatedNamespaces := map[string]string{}

	relatedNamespaces[o.config.WatchNamespace] = ""
	relatedNamespaces[o.config.LeaderElectionNamespace] = ""
	relatedNamespaces[o.config.ClusterAutoscalerNamespace] = ""

	relatedObjects := []configv1.ObjectReference{}

	for namespace := range relatedNamespaces {
		relatedObjects = append(relatedObjects, configv1.ObjectReference{
			Resource: "namespaces",
			Name:     namespace,
		})
	}

	return relatedObjects
}

// AddControllers configures the various controllers and adds them to
// the operator's manager instance.
func (o *Operator) AddControllers() error {
	// Setup ClusterAutoscaler controller.
	ca := clusterautoscaler.NewReconciler(o.manager, &clusterautoscaler.Config{
		ReleaseVersion: o.config.ReleaseVersion,
		Name:           o.config.ClusterAutoscalerName,
		Image:          o.config.ClusterAutoscalerImage,
		Replicas:       o.config.ClusterAutoscalerReplicas,
		Namespace:      o.config.ClusterAutoscalerNamespace,
		CloudProvider:  o.config.ClusterAutoscalerCloudProvider,
		Verbosity:      o.config.ClusterAutoscalerVerbosity,
		ExtraArgs:      o.config.ClusterAutoscalerExtraArgs,
	})

	if err := ca.AddToManager(o.manager); err != nil {
		return err
	}

	// Setup MachineAutoscaler controller.
	ma := machineautoscaler.NewReconciler(o.manager, &machineautoscaler.Config{
		Namespace:           o.config.ClusterAutoscalerNamespace,
		SupportedTargetGVKs: machineautoscaler.DefaultSupportedTargetGVKs(),
	})

	if err := ma.AddToManager(o.manager); err != nil {
		return err
	}

	return nil
}

// Start starts the operator's controller-manager.
func (o *Operator) Start() error {
	stopCh := signals.SetupSignalHandler()

	return o.manager.Start(stopCh)
}

// AddWebhooks sets up the webhook server, registers handlers, and adds the
// server to operator's manager instance.
func (o *Operator) AddWebhooks() error {
	// Set the CA certificate on the webhook configs.
	if err := o.setWebhookCA(); err != nil {
		return err
	}

	server := &webhook.Server{
		Port:    o.config.WebhooksPort,
		CertDir: o.config.WebhooksCertDir,
	}

	return o.manager.Add(server)
}

// setWebhookCA sets the caBundle field on the admission webhook configuration
// objects associated with the operator.
//
// Hopefully at some point the service-ca-operator will support injection for
// webhook configurations, at which point this will no longer be necessary.
func (o *Operator) setWebhookCA() error {
	config := o.manager.GetConfig()
	client, err := admissionregistrationv1beta1.NewForConfig(config)
	if err != nil {
		return err
	}

	ca, err := o.getEncodedCA()
	if err != nil {
		return err
	}

	// TODO: This should probably replace the caBundle in all webhook client
	// configurations in the object, but unfortuntaely that's not easy to do
	// with a JSON patch.  For now this only modifies the first entry.
	patch := []jsonpatch.Operation{
		{
			Operation: "replace",
			Path:      "/webhooks/0/clientConfig/caBundle",
			Value:     ca,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	_, err = client.ValidatingWebhookConfigurations().
		Patch("autoscaling.openshift.io", types.JSONPatchType, patchBytes)
	if err != nil {
		return err
	}

	return nil
}

// getEncodedCA returns the base64 encoded CA certificate used for securing
// admission webhook server connections.
func (o *Operator) getEncodedCA() (string, error) {
	caPath := filepath.Join(o.config.WebhooksCertDir, "service-ca", "ca-cert.pem")

	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ca), nil
}
