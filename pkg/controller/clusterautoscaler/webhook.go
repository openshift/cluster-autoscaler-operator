package clusterautoscaler

import (
	"context"
	"net/http"

	autoscalingv1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

// ValidatingWebhookName is the name used in the configuration for the
// ClusterAutoscaler validating webhook.
const ValidatingWebhookName = "clusterautoscalers.autoscaling.openshift.io"

// NewValidatingWebhook returns a new validating webhook for ClusterAutoscalers.
func NewValidatingWebhook(mgr manager.Manager) (*admission.Webhook, error) {
	operations := []admissionregistrationv1beta1.OperationType{
		admissionregistrationv1beta1.Create,
		admissionregistrationv1beta1.Update,
	}

	caValidator := &validator{}

	return builder.NewWebhookBuilder().
		Name(ValidatingWebhookName).
		Validating().
		Operations(operations...).
		WithManager(mgr).
		ForType(&autoscalingv1.ClusterAutoscaler{}).
		Handlers(caValidator).
		Build()
}

type validator struct {
	client  client.Client
	decoder types.Decoder
}

// validator implements the admission.Handler interface.
var _ admission.Handler = &validator{}

func (v *validator) Handle(ctx context.Context, req types.Request) types.Response {
	ca := &autoscalingv1.ClusterAutoscaler{}

	if err := v.decoder.Decode(req, ca); err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	klog.Infof("Validation webhook called for ClustAutoscaler: %s", ca.GetName())

	allowed, reason, err := true, "NOT YET IMPLEMENTED", error(nil)
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	return admission.ValidationResponse(allowed, reason)
}

var _ inject.Client = &validator{}

// InjectClient injects the client.
func (v *validator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

var _ inject.Decoder = &validator{}

// InjectDecoder injects the decoder.
func (v *validator) InjectDecoder(d types.Decoder) error {
	v.decoder = d
	return nil
}
