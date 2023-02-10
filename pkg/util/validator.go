/*
Copyright 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package util

import (
	"fmt"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// Warning for when the accelerator label does not exist.
	GPUAcceleratorLabelAbsentWarning = "GPU accelerator label not found on %s %s. "

	// Warning for when the accelerator label is present but is empty.
	GPUAcceleratorLabelEmptyStringWarning = "GPU accelerator label is empty. "

	// Warning for when the accelerator label is present but is poorly formed.
	GPUAcceleratorLabelPoorlyFormedWarning = "GPU accelerator label contains a poorly formed value: %s, errors: %s. "

	// Link to KCS article about fixing the issue, this should be added to the end of any warning.
	GPUAcceleratorLabelKCSWarning = "This is not an error but may cause issues when using GPU resource limits with the Cluster Autoscaler. For more information on the proper use of these values, please see https://access.redhat.com/solutions/6055181"
)

// ValidatorResponse represents the results of a validation request.
type ValidatorResponse struct {
	Warnings []string
	Errors   utilerrors.Aggregate
}

// IsValid tests a ValidationResponse to determine if the response was free from errors.
func (vr ValidatorResponse) IsValid() bool {
	return vr.Errors == nil || len(vr.Errors.Errors()) == 0
}

// IsValidGPUAcceleratorLabel tests whether the value passed is a
// valid for the GPU accelerator label on nodes. If the value is
// invalid, or empty, it returns a string with a relevant warning about the
// value. Otherwise an empty string is returned.
func IsValidGPUAcceleratorLabel(target string) string {
	var warning string

	if len(target) == 0 {
		warning = GPUAcceleratorLabelEmptyStringWarning
	} else if errs := validation.IsValidLabelValue(target); len(errs) > 0 {
		// concatenate the strings from IsValidLabelValue() as it can return multiple syntax errors
		warning = fmt.Sprintf(GPUAcceleratorLabelPoorlyFormedWarning, target, strings.Join(errs, ","))
	}

	if len(warning) > 0 {
		warning += GPUAcceleratorLabelKCSWarning
	}

	return warning
}
