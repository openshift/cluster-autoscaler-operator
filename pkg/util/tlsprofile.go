package util

import (
	"context"
	"crypto/tls"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	tlspkg "github.com/openshift/controller-runtime-common/pkg/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

// FetchClusterTLSProfile retrieves the TLS security profile from the APIServer
// cluster config object.
// Returns the fetched (or default) TLS profile and a TLS profile function
func FetchClusterTLSProfile(ctx context.Context, clientConfig *rest.Config) (configv1.TLSProfileSpec, []func(*tls.Config), error) {
	// Use a typed client since the manager is not yet started.
	configClient, err := configv1client.NewForConfig(clientConfig)
	if err != nil {
		return configv1.TLSProfileSpec{}, nil, fmt.Errorf("Unable to create TLS profile. Failed to create config client: %w", err)
	}

	apiServer, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return configv1.TLSProfileSpec{}, nil, fmt.Errorf("Unable to create TLS profile. Failed to get APIServer \"cluster\": %w", err)
	}

	profileSpec, err := tlspkg.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile) // also will return a default profile if not specified
	if err != nil {
		return configv1.TLSProfileSpec{}, nil, fmt.Errorf("Unable to create TLS profile. Failed to resolve TLS profile spec: %w", err)
	}

	tlsConfigFn, unsupported := tlspkg.NewTLSConfigFromProfile(profileSpec)
	if len(unsupported) > 0 {
		klog.Warningf("Ignoring unsupported ciphersuites from TLS profile: %v", unsupported)
	}

	profileType := configv1.TLSProfileIntermediateType
	if apiServer.Spec.TLSSecurityProfile != nil {
		profileType = apiServer.Spec.TLSSecurityProfile.Type
	}
	klog.Infof("Using cluster TLS profile %q (min version: %s) for TLS", profileType, profileSpec.MinTLSVersion)
	return profileSpec, []func(*tls.Config){tlsConfigFn}, nil
}

// SetupTLSProfileWatcher registers a controller with mgr to watch the APIServer object's TLS security profile for changes.
// If the profile changes, the cancel function will be called so that the operator can gracefully shutdown and restart to
// pick up the changes
func SetupTLSProfileWatcher(mgr ctrl.Manager, initialProfile configv1.TLSProfileSpec, cancel context.CancelFunc) error {
	watcher := &tlspkg.SecurityProfileWatcher{
		Client:                mgr.GetClient(),
		InitialTLSProfileSpec: initialProfile,
		OnProfileChange: func(ctx context.Context, oldSpec, newSpec configv1.TLSProfileSpec) {
			mgr.GetLogger().Info("TLS profile changed, triggering shutdown to reload",
				"old", oldSpec.MinTLSVersion, "new", newSpec.MinTLSVersion)
			cancel()
		},
	}
	return watcher.SetupWithManager(mgr)
}
