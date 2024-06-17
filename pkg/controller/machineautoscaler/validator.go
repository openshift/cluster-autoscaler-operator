package machineautoscaler

import (
	"context"
	"errors"
	"net/http"

	autoscalingv1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	"github.com/openshift/cluster-autoscaler-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator validates MachineAutoscaler resources.
type Validator struct {
	client  client.Client
	decoder admission.Decoder
}

// NewValidator returns a new Validator.
func NewValidator(client client.Client, scheme *runtime.Scheme) *Validator {
	return &Validator{
		client:  client,
		decoder: admission.NewDecoder(scheme),
	}
}

// Validate validates the given MachineAutoscaler resource.
func (v *Validator) Validate(ma *autoscalingv1beta1.MachineAutoscaler) util.ValidatorResponse {
	var errs []error

	if ma == nil {
		err := errors.New("MachineAutoscaler is nil")
		return util.ValidatorResponse{Warnings: nil, Errors: utilerrors.NewAggregate([]error{err})}
	}

	if ma.Spec.MinReplicas < 0 || ma.Spec.MaxReplicas < 0 {
		errs = append(errs, errors.New("min and max replicas must be greater than 0"))
	}

	if ma.Spec.MaxReplicas < ma.Spec.MinReplicas {
		errs = append(errs, errors.New("max replicas must be greater than or equal to min"))
	}

	if len(errs) > 0 {
		return util.ValidatorResponse{Warnings: nil, Errors: utilerrors.NewAggregate(errs)}
	}

	return util.ValidatorResponse{}
}

// Handle handles HTTP requests for admission webhook servers.
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	ma := &autoscalingv1beta1.MachineAutoscaler{}

	if err := v.decoder.Decode(req, ma); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.Infof("Validation webhook called for MachineAutoscaler: %s", ma.GetName())

	var admRes admission.Response

	valRes := v.Validate(ma)
	if valRes.IsValid() {
		admRes = admission.Allowed("MachineAutoscaler valid")
	} else {
		admRes = admission.Denied(valRes.Errors.Error())
	}

	if len(valRes.Warnings) > 0 {
		admRes = admRes.WithWarnings(valRes.Warnings...)
	}

	return admRes
}

// InjectClient injects the client.
func (v *Validator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}
