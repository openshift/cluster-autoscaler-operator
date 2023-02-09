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
	"testing"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestValidatorResponseIsValid(t *testing.T) {
	testConfigs := []struct {
		name     string
		warnings []string
		errors   utilerrors.Aggregate
		expected bool
	}{
		{
			name:     "No warnings and errors is valid",
			warnings: nil,
			errors:   nil,
			expected: true,
		},
		{
			name:     "Some warnings and no errors is valid",
			warnings: []string{"foo", "bar"},
			errors:   nil,
			expected: true,
		},
		{
			name:     "No warnings and some errors is not valid",
			warnings: nil,
			errors:   utilerrors.NewAggregate([]error{fmt.Errorf("foo")}),
			expected: false,
		},
		{
			name:     "Some warnings and some errors is not valid",
			warnings: []string{"foo", "bar"},
			errors:   utilerrors.NewAggregate([]error{fmt.Errorf("foo")}),
			expected: false,
		},
	}

	for _, tc := range testConfigs {
		t.Run(tc.name, func(t *testing.T) {
			vr := ValidatorResponse{Warnings: tc.warnings, Errors: tc.errors}
			observed := vr.IsValid()
			if observed != tc.expected {
				t.Errorf("Expected %v, got %v", observed, tc.expected)
			}
		})
	}
}
