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
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

const (
	errorVersion = "0.0.0-unable-to-set-version"
)

var (
	// Raw is the string representation of the version. This will be replaced
	// with the calculated version at build time.
	Raw = "0.0.0-version-not-set"

	// Version is semver representation of the version.
	Version = mustParse(Raw)

	// String is the human-friendly representation of the version.
	String = fmt.Sprintf("cluster-autoscaler-operator %s", Version)
)

func mustParse(v string) semver.Version {
	if version, err := semver.Parse(strings.TrimLeft(v, "v")); err != nil {
		// version did not parse cleanly, append it to the zero value
		appendedVersion, err := semver.Parse(fmt.Sprintf("0.0.0-%s", v))
		if err != nil {
			// still having an issue making a clean version, use a known value
			// this shouldn't happen in practice
			return semver.MustParse(errorVersion)
		}

		return appendedVersion
	} else {
		return version
	}
}
