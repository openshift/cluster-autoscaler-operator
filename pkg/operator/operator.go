package operator

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller/clusterautoscaler"
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller/machineautoscaler"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
	"github.com/openshift/library-go/pkg/operator/events"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// for use with release version detection
const (
	releaseVersionEnvVariableName = "RELEASE_VERSION"
	unknownVersionValue           = "unknown"
)

// OperatorName is the name of this operator.
const OperatorName = "cluster-autoscaler"

// Operator represents an instance of the cluster-autoscaler-operator.
type Operator struct {
	config  *Config
	manager manager.Manager

	caReconciler        *clusterautoscaler.Reconciler
	maReconciler        *machineautoscaler.Reconciler
	FeatureGateAccessor featuregates.FeatureGateAccess
}

// New returns a new Operator instance with the given config and a
// manager configured with the various controllers.
func New(stopCh context.Context, cfg *Config) (*Operator, error) {
	operator := &Operator{config: cfg}

	// Get a config to talk to the apiserver.
	clientConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	// Get defaults for leader election
	le := util.GetLeaderElectionDefaults(clientConfig, configv1.LeaderElection{
		Disable:   !cfg.LeaderElection,
		Namespace: cfg.LeaderElectionNamespace,
		Name:      cfg.LeaderElectionID,
	})

	// Create the controller-manager.
	managerOptions := manager.Options{
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				cfg.WatchNamespace: {},
			},
		},
		LeaderElection:                cfg.LeaderElection,
		LeaderElectionNamespace:       cfg.LeaderElectionNamespace,
		LeaderElectionID:              cfg.LeaderElectionID,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &le.LeaseDuration.Duration,
		RenewDeadline:                 &le.RenewDeadline.Duration,
		RetryPeriod:                   &le.RetryPeriod.Duration,
		Metrics: server.Options{
			BindAddress: fmt.Sprintf("127.0.0.1:%d", cfg.MetricsPort),
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				TLSOpts: []func(*tls.Config){
					func(cfg *tls.Config) {
						cfg.MinVersion = tls.VersionTLS13
					},
				},
			},
		},
	}

	operator.manager, err = manager.New(clientConfig, managerOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %v", err)
	}

	// Setup Scheme for all resources.
	if err := apis.AddToScheme(operator.manager.GetScheme()); err != nil {
		return nil, fmt.Errorf("failed to register types: %v", err)
	}

	if err := monitoringv1.AddToScheme(operator.manager.GetScheme()); err != nil {
		return nil, fmt.Errorf("failed to register monitoring types: %v", err)
	}

	if err := configv1.AddToScheme(operator.manager.GetScheme()); err != nil {
		return nil, fmt.Errorf("failed to register configv1 types: %v", err)
	}

	// this needs to happen before the controllers so that we can configure them
	// with feature gate access.
	if featureGateAccessor, err := getFeatureGateAccessor(stopCh, operator); err != nil {
		return nil, fmt.Errorf("failed to get feature gate accessor: %w", err)
	} else {
		operator.FeatureGateAccessor = featureGateAccessor
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

	if err := operator.manager.Add(statusReporter); err != nil {
		return nil, fmt.Errorf("failed to add status reporter to manager: %v", err)
	}

	return operator, nil
}

// RelatedObjects returns a list of objects related to the operator and its
// operands.  These are used in the ClusterOperator status.
func (o *Operator) RelatedObjects() []configv1.ObjectReference {
	relatedNamespaces := map[string]string{}

	relatedNamespaces[o.config.WatchNamespace] = ""
	relatedNamespaces[o.config.LeaderElectionNamespace] = ""
	relatedNamespaces[o.config.ClusterAutoscalerNamespace] = ""

	// Related objects lets openshift/must-gather collect diagnostic content
	relatedObjects := []configv1.ObjectReference{
		{
			Group:     v1beta1.SchemeGroupVersion.Group,
			Resource:  "machineautoscalers",
			Name:      "",
			Namespace: o.config.WatchNamespace,
		},
		{
			Group:     v1beta1.SchemeGroupVersion.Group,
			Resource:  "clusterautoscalers",
			Name:      "",
			Namespace: o.config.WatchNamespace,
		},
	}

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
	ca := clusterautoscaler.NewReconciler(o.manager, clusterautoscaler.Config{
		ReleaseVersion:      o.config.ReleaseVersion,
		Name:                o.config.ClusterAutoscalerName,
		Image:               o.config.ClusterAutoscalerImage,
		Replicas:            o.config.ClusterAutoscalerReplicas,
		Namespace:           o.config.ClusterAutoscalerNamespace,
		CloudProvider:       o.config.ClusterAutoscalerCloudProvider,
		Verbosity:           o.config.ClusterAutoscalerVerbosity,
		ExtraArgs:           o.config.ClusterAutoscalerExtraArgs,
		FeatureGateAccessor: o.FeatureGateAccessor,
	})

	if err := ca.AddToManager(o.manager); err != nil {
		return err
	}

	// Setup MachineAutoscaler controller.
	ma := machineautoscaler.NewReconciler(o.manager, machineautoscaler.Config{
		Namespace:           o.config.ClusterAutoscalerNamespace,
		SupportedTargetGVKs: machineautoscaler.DefaultSupportedTargetGVKs(),
	})

	if err := ma.AddToManager(o.manager); err != nil {
		return err
	}

	o.caReconciler = ca
	o.maReconciler = ma

	return nil
}

// AddWebhooks sets up the webhook server, registers handlers, and adds the
// server to operator's manager instance.  This expects the reconcilers to have
// been configured previously via the AddControllers() method.
func (o *Operator) AddWebhooks() error {
	namespace := o.config.WatchNamespace

	// Set up the webhook config updater and add it to the manager.  This will
	// update the webhook configurations when and if this instance becomes the
	// leader.
	webhookUpdater, err := NewWebhookConfigUpdater(o.manager, namespace)
	if err != nil {
		return err
	}

	if err := o.manager.Add(webhookUpdater); err != nil {
		return err
	}

	serverOpts := webhook.Options{
		Port:    o.config.WebhooksPort,
		CertDir: o.config.WebhooksCertDir,
	}
	server := webhook.NewServer(serverOpts)

	server.Register("/validate-clusterautoscalers",
		&webhook.Admission{Handler: o.caReconciler.Validator()})

	server.Register("/validate-machineautoscalers",
		&webhook.Admission{Handler: o.maReconciler.Validator()})

	return o.manager.Add(server)
}

// Start starts the operator's controller-manager.
func (o *Operator) Start(stopCh context.Context) error {

	return o.manager.Start(stopCh)
}

func getReleaseVersion() string {
	releaseVersion := os.Getenv(releaseVersionEnvVariableName)
	if len(releaseVersion) == 0 {
		releaseVersion = unknownVersionValue
		klog.Infof("%s environment variable is missing, defaulting to %q", releaseVersionEnvVariableName, unknownVersionValue)
	}
	return releaseVersion
}

func getFeatureGateAccessor(stopCh context.Context, o *Operator) (featuregates.FeatureGateAccess, error) {
	// Setup for the feature gate accessor. This reads and monitors feature gates
	// from the FeatureGate object status for the given version.
	desiredVersion := getReleaseVersion()
	missingVersion := "0.0.1-snapshot"

	configClient, err := configv1client.NewForConfig(o.manager.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to create config client for feature gate informers")
	}
	configInformers := configinformers.NewSharedInformerFactory(configClient, 10*time.Minute)

	kubeClient, err := kubernetes.NewForConfig(o.manager.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client for feature gate informers")
	}

	controllerRef, err := events.GetControllerReferenceForCurrentPod(stopCh, kubeClient, o.config.ClusterAutoscalerNamespace, nil)
	if err != nil {
		klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
	}
	mgrClock := clock.RealClock{}
	recorder := events.NewKubeRecorder(kubeClient.CoreV1().Events(o.config.ClusterAutoscalerNamespace), "cluster-autoscaler-operator", controllerRef, mgrClock)
	featureGateAccessor := featuregates.NewFeatureGateAccess(
		desiredVersion, missingVersion,
		configInformers.Config().V1().ClusterVersions(), configInformers.Config().V1().FeatureGates(),
		recorder,
	)

	go featureGateAccessor.Run(stopCh)
	go configInformers.Start(stopCh.Done())

	select {
	case <-featureGateAccessor.InitialFeatureGatesObserved():
		features, _ := featureGateAccessor.CurrentFeatureGates()
		klog.Infof("FeatureGates initialized: %v", features.KnownFeatures())
	case <-time.After(1 * time.Minute):
		klog.Error(errors.New("timed out waiting for FeatureGate detection"), "unable to start manager")
		return nil, fmt.Errorf("time out waiting for FeatureGate detection")
	}

	return featureGateAccessor, nil
}
