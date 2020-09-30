package machineautoscaler

import (
	"context"
	"errors"
	"net/http"

	autoscalingv1beta1 "github.com/openshift/cluster-autoscaler-operator/pkg/apis/autoscaling/v1beta1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator validates MachineAutoscaler resources.
type Validator struct {
	client  client.Client
	decoder *admission.Decoder
}

// NewValidator returns a new Validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the given MachineAutoscaler resource.
func (v *Validator) Validate(ma *autoscalingv1beta1.MachineAutoscaler) (bool, utilerrors.Aggregate) {
	var errs []error

	if ma == nil {
		err := errors.New("MachineAutoscaler is nil")
		return false, utilerrors.NewAggregate([]error{err})
	}

	if ma.Spec.MinReplicas < 0 || ma.Spec.MaxReplicas < 0 {
		errs = append(errs, errors.New("min and max replicas must be greater than 0"))
	}

	if ma.Spec.MaxReplicas < ma.Spec.MinReplicas {
		errs = append(errs, errors.New("max replicas must be greater than or equal to min"))
	}

	if len(errs) > 0 {
		return false, utilerrors.NewAggregate(errs)
	}

	return true, nil
}

// Handle handles HTTP requests for admission webhook servers.
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	ma := &autoscalingv1beta1.MachineAutoscaler{}

	if err := v.decoder.Decode(req, ma); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.Infof("Validation webhook called for MachineAutoscaler: %s", ma.GetName())

	if ok, err := v.Validate(ma); !ok {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("MachineAutoscaler valid")
}

// InjectClient injects the client.
func (v *Validator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
