/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2026 Red Hat, Inc.
 *
 */
package version

import (
	"testing"

	"github.com/blang/semver/v4"
)

func TestMustParse(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected semver.Version
	}{
		{
			name:     "v0.0.0 parses properly",
			input:    "v0.0.0",
			expected: semver.MustParse("0.0.0"),
		},
		{
			name:     "abbreviated commit hash returns concatenated version",
			input:    "abcdef",
			expected: semver.MustParse("0.0.0-abcdef"),
		},
		{
			name:     "malformed returns well known error",
			input:    "",
			expected: semver.MustParse(errorVersion),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := mustParse(tc.input)
			if !observed.Equals(tc.expected) {
				t.Errorf("versions do not match, expected %q, observed %q", tc.expected, observed)
			}
		})
	}
}
