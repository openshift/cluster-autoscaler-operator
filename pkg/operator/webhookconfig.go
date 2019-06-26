package operator

import (
	"context"
	"encoding/base64"
	"io/ioutil"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// WebhookConfigurationName is the name of the webhook configurations to be
// updated with the current CA certificate.
const WebhookConfigurationName = "autoscaling.openshift.io"

// WebhookConfigUpdater updates webhook configurations to point the Kubernetes
// API server at the operator's validating or mutating webhook server.  It would
// be nice if the CVO could apply the configuration as it is mostly static.
// Unfortunately, the CA bundle is not known until runtime.
type WebhookConfigUpdater struct {
	caPath    string
	namespace string
	client    client.Client
}

// NewWebhookConfigUpdater returns a new WebhookConfigUpdater instance.
func NewWebhookConfigUpdater(mgr manager.Manager, namespace, caPath string) (*WebhookConfigUpdater, error) {
	w := &WebhookConfigUpdater{
		caPath:    caPath,
		namespace: namespace,
		client:    mgr.GetClient(),
	}

	return w, nil
}

// Start creates or updates the webhook configurations then waits for the stop
// channel to be closed.
func (w *WebhookConfigUpdater) Start(stopCh <-chan struct{}) error {
	vc := &admissionregistrationv1beta1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1beta1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: WebhookConfigurationName,
			Labels: map[string]string{
				"k8s-app": OperatorName,
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), w.client, vc, func() error {
		var err error
		vc.Webhooks, err = w.ValidatingWebhooks()
		return err
	})

	if err != nil {
		return err
	}

	klog.Infof("Webhook configuration status: %s", op)

	// Block until the stop channel is closed.
	<-stopCh

	return nil
}

// ValidatingWebhooks returns the validating webhook configurations.
func (w *WebhookConfigUpdater) ValidatingWebhooks() ([]admissionregistrationv1beta1.Webhook, error) {
	caBundle, err := w.GetEncodedCA()
	if err != nil {
		return nil, err
	}

	failurePolicy := admissionregistrationv1beta1.Ignore
	sideEffects := admissionregistrationv1beta1.SideEffectClassNone

	webhooks := []admissionregistrationv1beta1.Webhook{
		{
			Name: "clusterautoscalers.autoscaling.openshift.io",
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Name:      OperatorName,
					Namespace: w.namespace,
					Path:      pointer.StringPtr("/validate-clusterautoscalers"),
				},
				CABundle: []byte(caBundle),
			},
			FailurePolicy: &failurePolicy,
			SideEffects:   &sideEffects,
			Rules: []admissionregistrationv1beta1.RuleWithOperations{
				{
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{"autoscaling.openshift.io"},
						APIVersions: []string{"v1"},
						Resources:   []string{"clusterautoscalers"},
					},
					Operations: []admissionregistrationv1beta1.OperationType{
						admissionregistrationv1beta1.Create,
						admissionregistrationv1beta1.Update,
					},
				},
			},
		},
	}

	return webhooks, nil
}

// GetEncodedCA returns the base64 encoded CA certificate used for securing
// admission webhook server connections.
func (w *WebhookConfigUpdater) GetEncodedCA() (string, error) {
	ca, err := ioutil.ReadFile(w.caPath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ca), nil
}
