---
description: Automate Cluster Autoscaler Operator release version bumps and dependency updates
argument-hint: <new_ocp_version> [new_go_version] [dry_run]
allowed-tools: Read, Edit, Glob, Grep, Bash(curl:*), Bash(go get:*), Bash(go list:*), Bash(go mod tidy:*), Bash(go mod vendor:*), Bash(git checkout:*), Bash(git add:*), Bash(git commit:*), Bash(git show:*), Bash(git log:*), Bash(git status:*), Bash(git diff:*), Bash(git branch:*), Bash(grep:*), Bash(sed:*), Bash(make build:*), Bash(make test:*), Bash(make check:*)
---

# Cluster Autoscaler Operator Release Chores

This guide is designed to be equally useful for a human or an agent performing the release chore cycle. Humans can follow the steps manually; agents should execute them in order. Each step describes what to do and why.

## Parameters

Arguments are positional:

1. `$1` - **new_ocp_version** (required): New OpenShift version (e.g., `5.1` or `4.22`)
2. `$2` - **new_go_version** (optional): New Go minor version (e.g., `1.27`). If omitted, we use the Go version associated with the corresponding Kubernetes version — this is the recommended default.
3. `$3` - **dry_run** (optional): Set to `true` to preview changes without applying (default: `false`)

## Background

Each release cycle, we bump the Go toolchain, update Kubernetes and OpenShift dependency versions, and re-vendor. The ART (Automated Release Tooling) team separately handles updating `.ci-operator.yaml` and `Dockerfile.rhel` via automated PRs. However, we can do the chores before ART's PR lands — this skill updates those files too, so ART's PR becomes a no-op if we get there first.

## Your Task

Execute the following steps to perform the release chores:

### Step 1: Pre-flight Checks

1. Parse and validate arguments:
   - `NEW_OCP_VERSION="$1"` (required, e.g., `5.1`)
   - `NEW_GO_VERSION="$2"` (optional, e.g., `1.27` — will be auto-detected if omitted)
   - `DRY_RUN="${3:-false}"` (optional, default: false)
   - Note: if `$2` is `true` or `false`, treat it as `DRY_RUN` and leave `NEW_GO_VERSION` empty for auto-detection

2. Validate inputs:
   - Ensure `NEW_OCP_VERSION` is provided and matches format `X.Y` (e.g., `5.1` or `4.22`)
   - If `NEW_GO_VERSION` is provided, ensure it matches format `X.Y` (e.g., `1.27`)
   - If `DRY_RUN` is provided, ensure it's `true` or `false`

3. Verify environment:
   - Check that `Dockerfile`, `Makefile`, and `go.mod` exist (confirms correct repository)
   - Check for uncommitted changes with `git status` and warn user if any exist
   - If uncommitted changes exist, **STOP** and ask user to commit or stash first

### Step 2: Detect Current Versions

Extract current versions from files:

1. **Current Go version (Dockerfile)**: Extract from `Dockerfile` using the `golang-X.Y` pattern on the `FROM` line
2. **Current Go version (Makefile)**: Extract from `Makefile` using the `golang-X.Y` pattern in `BUILD_IMAGE`
3. **Current Go version (go.mod)**: Extract from `go.mod` using the `^go` line
4. **Current OCP version**: Extract from `Dockerfile.rhel` using the `openshift-X.Y` pattern on the builder `FROM` line and the `ocp/X.Y:base` pattern on the base `FROM` line
5. **Current k8s version**: Extract from `go.mod` using the `k8s.io/api` version in the require block
6. **Current ENVTEST_K8S_VERSION**: Extract from `Makefile` using the `ENVTEST_K8S_VERSION` variable

Display all detected versions to the user.

### Step 3: Determine New Kubernetes Version

Find the latest stable (non-alpha, non-beta, non-rc) Kubernetes package version:

```bash
K8SVER=$(go list -mod=readonly -m -versions k8s.io/api | sed 's/ /\n/g' | grep -v 'alpha\|beta\|rc' | sort -rV | head -1)
echo "New k8s version: $K8SVER"
```

Extract the minor version number (used for both ENVTEST and Go version detection):

```bash
K8S_MINOR=$(echo $K8SVER | sed 's/^v0\.\([0-9]*\).*/\1/')
```

Compute the new ENVTEST version (k8s.io/api v0.X.Y corresponds to Kubernetes 1.X.Y):

```bash
NEW_ENVTEST_VERSION=$(echo $K8SVER | sed 's/^v0\./1./')
```

### Step 4: Detect or Validate Go Version

If `NEW_GO_VERSION` was not provided, use the Go version associated with the corresponding Kubernetes version:

```bash
NEW_GO_VERSION=$(curl -sL "https://raw.githubusercontent.com/kubernetes/kubernetes/v1.${K8S_MINOR}.0/go.mod" | grep '^go ' | awk '{print $2}')
```

This extracts the `go` directive from the Kubernetes `go.mod` for the matching release. For example, Kubernetes 1.36 uses Go 1.26.

If `NEW_GO_VERSION` was provided as an argument, use it as-is but display the auto-detected value for comparison.

Display the Go version to the user and confirm.

### Step 5: Derive Controller-Runtime Version

Controller-runtime's minor version tracks 12 behind the k8s.io/api minor version (e.g., k8s.io/api v0.36.x → controller-runtime v0.24.x):

```bash
CR_MINOR=$((K8S_MINOR - 12))
CRVER=$(go list -mod=readonly -m -versions sigs.k8s.io/controller-runtime | sed 's/ /\n/g' | grep -v 'alpha\|beta\|rc' | grep "^v0\.${CR_MINOR}\." | sort -rV | head -1)
echo "New controller-runtime version: $CRVER"
```

Display the detected version and ask user to confirm, or allow override.

### Step 6: Display Change Summary

Present a summary of all planned changes:

```
Go version:
  Dockerfile:          [CURRENT] → [NEW_GO_VERSION]
  Makefile:            [CURRENT] → [NEW_GO_VERSION]
  go.mod:              [CURRENT] → [NEW_GO_VERSION].0

Kubernetes:
  k8s.io/* deps:       [CURRENT] → [K8SVER]
  controller-runtime:  [CURRENT] → [CRVER]
  ENVTEST_K8S_VERSION: [CURRENT] → [NEW_ENVTEST_VERSION]

OpenShift:
  OCP version:         [CURRENT] → [NEW_OCP_VERSION]
  OpenShift deps:      will be updated to latest

Files to be modified:
  - Dockerfile (golang builder version)
  - Dockerfile.rhel (builder and base image OCP/Go versions)
  - .ci-operator.yaml (build root image tag)
  - Makefile (BUILD_IMAGE, ENVTEST_K8S_VERSION)
  - go.mod (go version, dependencies)
  - go.sum (via go mod tidy)
  - vendor/ (via go mod vendor)
```

**If `DRY_RUN` is `true`**: Display summary and exit without making changes.

**Otherwise**: Ask user for confirmation before proceeding.

### Step 7: Update Dockerfile

Update the Go builder image version in `Dockerfile`:

Replace the `golang-X.Y` version in the `FROM` line:
```
FROM registry.ci.openshift.org/openshift/release:golang-$NEW_GO_VERSION AS builder
```

### Step 8: Update Dockerfile.rhel

Update both image references in `Dockerfile.rhel`:

1. **Builder image**: Update the Go and OCP versions:
   ```
   FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-$NEW_GO_VERSION-openshift-$NEW_OCP_VERSION AS builder
   ```

2. **Base image**: Update the OCP version:
   ```
   FROM registry.ci.openshift.org/ocp/$NEW_OCP_VERSION:base-rhel9
   ```

### Step 9: Update .ci-operator.yaml

Update the build root image tag:

```yaml
build_root_image:
  name: release
  namespace: openshift
  tag: rhel-9-release-golang-$NEW_GO_VERSION-openshift-$NEW_OCP_VERSION
```

### Step 10: Update Makefile

1. Update `BUILD_IMAGE` Go version:
   ```
   BUILD_IMAGE ?= registry.ci.openshift.org/openshift/release:golang-$NEW_GO_VERSION
   ```

2. Update `ENVTEST_K8S_VERSION`:
   ```
   ENVTEST_K8S_VERSION = $NEW_ENVTEST_VERSION
   ```

### Step 11: Update go.mod Go Version

Update the `go` directive in `go.mod`:
```
go $NEW_GO_VERSION.0
```

If a `toolchain` directive exists, update or remove it as appropriate.

### Step 12: Update Go Dependencies

Run the following commands in sequence:

1. **Update k8s, controller-runtime, and openshift dependencies**:
   ```bash
   go get k8s.io/api@$K8SVER k8s.io/apimachinery@$K8SVER k8s.io/client-go@$K8SVER sigs.k8s.io/controller-runtime@$CRVER github.com/openshift/api github.com/openshift/client-go github.com/openshift/library-go github.com/openshift/machine-api-operator
   ```
   - `controller-runtime` must be updated alongside k8s deps — its minor version tracks 12 behind the k8s minor version (e.g., k8s 0.36 → CR 0.24), and mismatches cause build failures.
   - If any `go get` fails, display the error and ask user how to proceed.

2. **Tidy modules**:
   ```bash
   go mod tidy
   ```

3. **Re-vendor**:
   ```bash
   go mod vendor
   ```

Each of these steps can take a few minutes. If any fails, **STOP** and report the error — the user may need to resolve dependency conflicts manually.

### Step 13: Verify Build

Run the build to verify everything compiles:

```bash
make build
```

If the build fails, report the errors. Common issues:
- API changes in updated dependencies may require code changes in `pkg/`
- If code changes are needed, make them and re-run `make build`

### Step 14: Run Tests

Run the test suite:

```bash
make test
```

If tests fail, investigate and fix. Updated dependencies sometimes introduce breaking changes that require adaptation.

### Step 15: Display Results

1. Show what changed:
   ```bash
   git status
   git diff --stat
   ```

2. Verify key files look correct:
   - Check `go.mod` has the right Go and k8s versions
   - Check `Dockerfile` has the right builder image
   - Check `Dockerfile.rhel` has the right builder and base images
   - Check `.ci-operator.yaml` has the right tag
   - Check `Makefile` has the right BUILD_IMAGE and ENVTEST_K8S_VERSION

3. Note on ART-managed files:
   - If `.ci-operator.yaml` and `Dockerfile.rhel` were updated by this skill, ART's automated PR (if it comes later) will be a no-op — no conflict.
   - If ART already merged their PR before this skill ran, verify the versions match what we set.

### Step 16: Next Steps (for the user)

After the chore changes are complete:

1. **Review all changes** before committing:
   ```bash
   git diff
   ```

2. **Commit the changes**:
   ```bash
   git add -A
   git commit -m "Update dependencies and Go $NEW_GO_VERSION for $NEW_OCP_VERSION"
   ```

3. **Push and create a PR**.

4. **Test CI**: Once the PR is open, verify CI passes. Also check the periodic pre-submit E2E by adding a comment:
   ```
   /test e2e-aws-periodic-pre
   ```

## Examples

```bash
# Update for OCP 5.1 — Go version auto-detected from k8s go.mod (recommended)
/release-chores 5.1

# Update for OCP 5.1 with explicit Go version
/release-chores 5.1 1.27

# Dry run — preview what would change
/release-chores 5.1 true

# Dry run with explicit Go version
/release-chores 5.1 1.27 true
```

## Troubleshooting

### "go get" fails for an openshift dependency

OpenShift modules use pseudo-versions pinned to specific commits. If `go get github.com/openshift/api` fails, it may be because the latest commit on the default branch isn't compatible yet. Try:
- Check what version other repos are using
- Pin to a specific commit: `go get github.com/openshift/api@<commit-sha>`

### "go mod tidy" fails

Usually means a dependency version conflict. Check:
- Whether the k8s version you chose is actually released: `go list -m -versions k8s.io/api`
- Whether openshift deps are compatible with that k8s version
- Reset and try a different version if needed: `git checkout -- go.mod go.sum vendor/`

### Build fails after dependency update

Updated dependencies may introduce API changes. Common fixes:
- Check the diff in vendored code to understand what changed
- Update call sites in `pkg/` to match new APIs
- Look at how other OpenShift operators handled the same update

### ART files already updated by ART

If ART's automated PR merged before you ran this skill, verify the versions in `.ci-operator.yaml` and `Dockerfile.rhel` match what we're setting. If they do, the edits in steps 8–9 are no-ops. If they don't, investigate whether ART used different versions.
