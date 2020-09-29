package operator

import (
	"context"
	"fmt"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// WebhookConfigurationName is the name of the webhook configuration.
const WebhookConfigurationName = "autoscaling.openshift.io"

// InjectCABundleAnnotationName is the annotation used by the
// service-ca-operator to indicate which resources it should inject the CA into.
const InjectCABundleAnnotationName = "service.beta.openshift.io/inject-cabundle"

// WebhookConfigUpdater updates webhook configurations to point the Kubernetes
// API server at the operator's validating or mutating webhook server.  It would
// be nice if the CVO could apply the configuration as it is mostly static.
// Unfortunately, the service-ca-operator needs to be able to inject the CA
// certificate bundle, which the CVO would overwrite.
type WebhookConfigUpdater struct {
	namespace string
	client    client.Client
}

// NewWebhookConfigUpdater returns a new WebhookConfigUpdater instance.
func NewWebhookConfigUpdater(mgr manager.Manager, namespace string) (*WebhookConfigUpdater, error) {
	w := &WebhookConfigUpdater{
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
				"k8s-app": fmt.Sprintf("%s-operator", OperatorName),
			},
			Annotations: map[string]string{
				InjectCABundleAnnotationName: "true",
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
func (w *WebhookConfigUpdater) ValidatingWebhooks() ([]admissionregistrationv1beta1.ValidatingWebhook, error) {
	failurePolicy := admissionregistrationv1beta1.Ignore
	sideEffects := admissionregistrationv1beta1.SideEffectClassNone

	webhooks := []admissionregistrationv1beta1.ValidatingWebhook{
		{
			Name: "clusterautoscalers.autoscaling.openshift.io",
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Name:      fmt.Sprintf("%s-operator", OperatorName),
					Namespace: w.namespace,
					Path:      pointer.StringPtr("/validate-clusterautoscalers"),
				},
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
		{
			Name: "machineautoscalers.autoscaling.openshift.io",
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Name:      fmt.Sprintf("%s-operator", OperatorName),
					Namespace: w.namespace,
					Path:      pointer.StringPtr("/validate-machineautoscalers"),
				},
			},
			FailurePolicy: &failurePolicy,
			SideEffects:   &sideEffects,
			Rules: []admissionregistrationv1beta1.RuleWithOperations{
				{
					Rule: admissionregistrationv1beta1.Rule{
						APIGroups:   []string{"autoscaling.openshift.io"},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"machineautoscalers"},
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
