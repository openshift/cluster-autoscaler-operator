# Contributing to Cluster Autoscaler Operator

This document covers contribution guidelines for the [Cluster Autoscaler Operator](https://github.com/openshift/cluster-autoscaler-operator), which manages deployments of the OpenShift Cluster Autoscaler. For architecture and usage details, see the [README](README.md).

## Related Resources

| Resource | Link |
|----------|------|
| Upstream Cluster Autoscaler | [openshift/kubernetes-autoscaler](https://github.com/openshift/kubernetes-autoscaler) |
| E2E Test Package | [openshift/cluster-api-actuator-pkg](https://github.com/openshift/cluster-api-actuator-pkg) |
| Machine API Operator | [openshift/machine-api-operator](https://github.com/openshift/machine-api-operator) |
| Developer Metrics Guide | [docs/dev/metrics.md](docs/dev/metrics.md) |
| User Alerts Guide | [docs/user/alerts.md](docs/user/alerts.md) |
| Example CRs | [examples/](examples/) |

## Getting Started

After forking the repository, run the following commands to set up your development environment:

```bash
# Install vendored dependencies
make vendor

# Verify the project builds
make build

# Run the full check suite (fmt, vet, lint, tests)
make check
```

If you are modifying API types in `pkg/apis/`, regenerate deepcopy and CRD files:

```bash
make generate
```

If you need to run the operator locally, set the required environment variable and start the binary:

```bash
export WATCH_NAMESPACE=openshift-machine-api
./bin/cluster-autoscaler-operator -alsologtostderr
```

## Review and Approval Policy

Every change in every pull request must be understood and approved by two humans. This can be the PR author and a reviewer, or — if the author used an AI tool and does not fully understand the contents of the PR — two human reviewers.

**Exception:** PRs authored by deterministic automation tools that are part of our CI and related systems (whose code has been reviewed by the OpenShift engineering org) can be merged with a single human review.

Every change should be closely scrutinized for bugs. Review changes from multiple angles:

- **API correctness**: Do changes to CRD types in `pkg/apis/` follow naming conventions, include proper validation markers, and maintain backward compatibility?
- **Reconciliation logic**: Is error handling correct? Are status updates and event recording consistent?
- **ClusterOperator status**: Could this change affect the operator's Available, Progressing, or Degraded conditions?
- **RBAC and security**: Are there new permissions required? Are changes to `install/03_rbac.yaml` or RBAC-related code properly scoped?
- **Webhook validation**: Are validating webhook changes correct and complete?
- **Generated artifacts**: Are CRD YAMLs and deepcopy files regenerated and up to date?

## PR Workflow

This repo uses [OpenShift CI (Prow)](https://docs.ci.openshift.org/) for continuous integration. PRs are automatically merged once all required tests pass and the correct labels are present.

### Required labels for merge

- `lgtm` — Added by a reviewer via the `/lgtm` command.
- `approved` — Added by an approver listed in the [OWNERS](OWNERS) file via the `/approve` command.

### Useful commands

Comment these on the PR:

| Command | Effect |
|---------|--------|
| `/lgtm` | Add the `lgtm` label after reviewing |
| `/lgtm cancel` | Remove the `lgtm` label |
| `/approve` | Add the `approved` label (OWNERS approvers only) |
| `/approve cancel` | Remove the `approved` label |
| `/hold` | Prevent the PR from being merged |
| `/hold cancel` | Remove the hold and allow merging |
| `/retest` | Re-run all failed required tests |
| `/retest-required` | Re-run only the failed required tests |
| `/test <test-name>` | Run a specific test |
| `/cc @username` | Request a review from a specific person |
| `/assign @username` | Assign someone to the PR |
| `/kind bug` | Label the PR as a bug fix |
| `/kind feature` | Label the PR as a feature |
| `/cherry-pick release-X.Y` | Create a cherry-pick PR to a release branch |

### LGTM and force-push

A new push to the PR branch removes the `lgtm` label. This is intentional — reviewers need to re-review after changes.

### Preventing premature merges

- Add the `WIP:` prefix to the PR title (e.g., `WIP: feat: work in progress`). Prow adds the `do-not-merge/work-in-progress` label automatically.
- Use `/hold` to temporarily block merging while awaiting additional review or testing.

## Test Expectations

### Unit tests

Required for all code changes. Run with:

```
make test
```

Unit tests use [Ginkgo](https://onsi.github.io/ginkgo/) as the test runner with [testify](https://github.com/stretchr/testify) for assertions. Controller tests use [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) to spin up a local Kubernetes API server. Race detection is enabled by default.

### E2E tests

E2E tests live in [openshift/cluster-api-actuator-pkg](https://github.com/openshift/cluster-api-actuator-pkg), not in this repo. Run them with:

```
make test-e2e
```

This requires a running OpenShift cluster with `KUBECONFIG` pointing to it. E2E tests are expected for new features or significant behavioral changes. If your change requires e2e coverage that does not yet exist, note it in the PR description and coordinate with reviewers.

## Verified Label

Use `/verified` to indicate changes have been verified. Examples:

```
/verified
/verified by e2e tests
/verified by unit tests
/verified deferred to QE
```

## Generated Code

The following files are generated and should never be hand-edited:

| File(s) | Generator | Regenerate with |
|---------|-----------|-----------------|
| `**/zz_generated.deepcopy.go` | controller-gen | `make gen-deepcopy` |
| `install/01_clusterautoscaler.crd.yaml` | controller-gen + OpenShift annotations | `make gen-crd` |
| `install/02_machineautoscaler.crd.yaml` | controller-gen + OpenShift annotations | `make gen-crd` |
| `install/11_provisioningrequest.crd.yaml` | controller-gen + OpenShift annotations | `make gen-crd` |
| `vendor/` | go mod vendor | `make vendor` |

After modifying API types or interfaces, regenerate and commit the results in the same PR:

```
make generate
```

This runs deepcopy generation, CRD generation, `goimports`, and verifies no uncommitted diff remains.

## Development Quick Reference

| Task | Command |
|------|---------|
| Build the operator binary | `make build` |
| Build container images | `make images` |
| Full build pipeline (build + images + check) | `make all` |
| Run unit tests | `make test` |
| Run e2e tests | `make test-e2e` |
| Run fmt + vet + lint + test | `make check` |
| Format code | `make fmt` |
| Run go vet | `make vet` |
| Run linter | `make lint` |
| Run goimports | `make goimports` |
| Generate deepcopy + CRDs + verify | `make generate` |
| Generate deepcopy only | `make gen-deepcopy` |
| Generate CRDs only | `make gen-crd` |
| Tidy and vendor dependencies | `make vendor` |

## Code Style

- Run `make check` before committing — it runs formatting, vetting, linting, and tests in one step.
- Follow Go conventions for error strings: lowercase, no trailing punctuation, wrap with `fmt.Errorf("context: %w", err)`.
- Use structured logging with klog: constant messages, key-value pairs in lowerCamelCase.
- Import ordering: stdlib, external packages, internal packages (separated by blank lines).
- All source files must include the Apache 2.0 license boilerplate (see `hack/boilerplate.go.txt`).

## Pre-Submit Checklist

Before requesting review:

1. `make build` — Verify the code compiles
2. `make check` — Run formatting, vetting, linting, and unit tests
3. If API types changed: `make generate` — Regenerate deepcopy and CRD files, commit the results
4. If dependencies changed: `make vendor` — Update vendor directory, commit the results
5. Review your diff for secrets, credentials, or debug code
6. New or changed behavior has unit test coverage
