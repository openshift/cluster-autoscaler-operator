# AGENTS.md — openshift/cluster-autoscaler-operator

This file provides AI-specific guidance for working in the Cluster Autoscaler Operator repository. For contribution guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Project Overview

This repo is the **Cluster Autoscaler Operator** for OpenShift — a standalone operator (`github.com/openshift/cluster-autoscaler-operator`) that manages the lifecycle of the Kubernetes Cluster Autoscaler on OpenShift clusters.

The operator is deployed by the Cluster Version Operator (CVO) into the `openshift-machine-api` namespace. It can also run on vanilla Kubernetes clusters with machine-api components available.

### Single Binary

This repo produces one binary:

| Binary | Source | Purpose |
|--------|--------|---------|
| `cluster-autoscaler-operator` | `cmd/manager/main.go` | The operator. Watches ClusterAutoscaler/MachineAutoscaler CRs and manages the cluster-autoscaler Deployment, monitoring, network policies, webhooks, and CVO status. |

### CRDs

Both CRDs belong to API group `autoscaling.openshift.io`.

| CRD | API Version | Scope | Purpose |
|-----|-------------|-------|---------|
| ClusterAutoscaler | `autoscaling.openshift.io/v1` | Cluster | Singleton (`"default"`) configuring the cluster-wide autoscaler: resource limits, scale-down/up settings, expanders, startup taints, GPU limits |
| MachineAutoscaler | `autoscaling.openshift.io/v1beta1` | Namespaced | Defines min/max replica bounds for a specific MachineSet target |

## Repository Structure

```
cmd/manager/                    # Single binary entry point
pkg/
  apis/autoscaling/v1/          # ClusterAutoscaler CRD types + deepcopy
  apis/autoscaling/v1beta1/     # MachineAutoscaler CRD types + deepcopy
  controller/clusterautoscaler/ # ClusterAutoscaler reconciler, monitoring, network policies, validation
  controller/machineautoscaler/ # MachineAutoscaler reconciler, MachineTarget management, validation
  operator/                     # Operator bootstrap, env-var config, CVO status reporter, webhook config
  util/                         # Shared utilities (validation, TLS profile, leader election, units)
  version/                      # Build-time version string
install/                        # CVO install manifests (CRDs, RBAC, Deployment, etc.), numbered for ordering
hack/                           # Build and CI scripts (test runner, CRD generation, formatting, vendoring)
test/helpers/                   # Shared test helper utilities (TestDeployment, TestClusterOperator builders)
examples/                       # Sample ClusterAutoscaler and MachineAutoscaler CR YAMLs
docs/dev/                       # Developer docs (metrics scraping)
docs/user/                      # User docs (Prometheus alerts)
.claude/commands/               # Claude Code slash commands (release-chores)
```

## Relationship to Upstream kubernetes-autoscaler

This operator does **not** contain any upstream autoscaler code. It is a standalone operator that manages a Deployment of the upstream cluster-autoscaler.

- The autoscaler operand lives in the [kubernetes-sigs/cluster-autoscaler](https://github.com/kubernetes-sigs/cluster-autoscaler).
- The operator's job: Runs the upstream autoscaler image.
- The autoscaler image is injected via the `CLUSTER_AUTOSCALER_IMAGE` env var (set by CVO from `install/image-references`).
- When working on this repo, you rarely need to touch upstream autoscaler code. Changes to autoscaler behavior are done via the CR spec fields here.

## Architecture: What Is Not Obvious

### Operator-Manages-Deployment Pattern

The operator creates and manages a separate Deployment (`cluster-autoscaler-<name>`) in the same namespace. It does not run the autoscaler in-process. The reconcile loop in `pkg/controller/clusterautoscaler/clusterautoscaler_controller.go` handles the full lifecycle: create, update, and delete the Deployment when the CR changes.

The operator also creates owned secondary resources:
- **Monitoring**: Service (port 8085), ServiceMonitor, PrometheusRule (4 alerts: UnschedulablePods, NotSafeToScale, CPULimitReached, MemoryLimitReached) — see `monitoring.go`
- **Network policies**: 5 NetworkPolicy objects (default-deny, DNS egress, API server egress, webhook ingress, metrics ingress) — see `networkpolicies.go`

### CVO Integration and Status Reporting

The CVO deploys the operator using numbered manifests in `install/`. The ordering matters (CRDs before RBAC before Deployment). The `StatusReporter` (`pkg/operator/status.go`) manages a `ClusterOperator` resource that reports Available, Progressing, and Degraded conditions to the CVO. It checks machine-api availability as a prerequisite and monitors the autoscaler Deployment health on a 15-second polling interval.

The `RELEASE_VERSION` env var controls version reporting. The operator signals `Progressing=True` on major/minor version changes and uses a degradation threshold (3 consecutive failures) to avoid flapping.

### Singleton Pattern

The `ClusterAutoscaler` CR is effectively a singleton. The operator only processes the CR whose name matches `CLUSTER_AUTOSCALER_NAME` (default: `"default"`). All other CRs with different names are silently ignored via a name predicate filter.

### MachineAutoscaler Annotation Model

`MachineAutoscaler` does not own or create MachineSets. Instead, it writes min/max annotations on existing MachineSet objects:
- `machine.openshift.io/cluster-api-autoscaler-node-group-min-size`
- `machine.openshift.io/cluster-api-autoscaler-node-group-max-size`

Ownership is tracked via the `autoscaling.openshift.io/machineautoscaler` annotation and a finalizer (`machinetarget.autoscaling.openshift.io`). When a MachineAutoscaler's target changes, the old target's annotations are cleaned up before the new target is configured.

### Environment Variable Configuration

All operator configuration comes from environment variables, parsed in `pkg/operator/config.go`. There are no config files or ConfigMaps.

| Variable | Default | Purpose |
|----------|---------|---------|
| `WATCH_NAMESPACE` | `openshift-machine-api` | Namespace to watch for CRs |
| `CLUSTER_AUTOSCALER_NAME` | `default` | Name of the ClusterAutoscaler CR to process |
| `CLUSTER_AUTOSCALER_IMAGE` | `quay.io/openshift/origin-cluster-autoscaler:v4.0` | Autoscaler container image |
| `CLUSTER_AUTOSCALER_NAMESPACE` | `openshift-machine-api` | Namespace for autoscaler Deployments |
| `CLUSTER_AUTOSCALER_CLOUD_PROVIDER` | `openshift` | Cloud provider name |
| `CLUSTER_AUTOSCALER_VERBOSITY` | `1` | Autoscaler log verbosity |
| `CLUSTER_AUTOSCALER_EXTRA_ARGS` | _(empty)_ | Extra CLI args (dev/debug only, not in CRD) |
| `RELEASE_VERSION` | _(empty)_ | Version reported to CVO |
| `WEBHOOKS_ENABLED` | `true` | Enable admission webhooks |
| `WEBHOOKS_PORT` | `8443` | Webhook server port |
| `WEBHOOKS_CERT_DIR` | `/etc/cluster-autoscaler-operator/tls` | TLS cert directory |
| `METRICS_PORT` | `8080` | Operator metrics port |
| `LEADER_ELECTION` | `true` | Enable leader election |

### Feature Gates and Cluster API

The operator reads OpenShift `FeatureGate` objects to conditionally enable Cluster API provider autodiscovery and ProvisioningRequest support. Whether Cluster API is enabled depends on per-platform feature gates (e.g., `ClusterAPIMachineManagementAWS` for AWS). When Cluster API is disabled for a platform, the operator sets `OPENSHIFT_CLUSTERAPI_DISABLE=true` on the autoscaler container.

### Validating Webhooks

The operator runs a webhook server (port 8443 by default) with validating webhooks for both CRDs at `/validate-clusterautoscalers` and `/validate-machineautoscalers`. The `WebhookConfigUpdater` (`pkg/operator/webhookconfig.go`) creates/updates the `ValidatingWebhookConfiguration` at startup. TLS certificates are managed by OpenShift's service-ca-operator.

## Build and Test Commands

| Task | Command |
|------|---------|
| Build the operator binary | `make build` |
| Run unit tests | `make test` |
| Run e2e tests (requires live cluster) | `make test-e2e` |
| Full check (fmt + vet + lint + test) | `make check` |
| Full build pipeline (build + images + check) | `make all` |
| Regenerate deepcopy + CRDs + verify | `make generate` |
| Generate deepcopy only | `make gen-deepcopy` |
| Generate CRDs only | `make gen-crd` |
| Tidy and vendor dependencies | `make vendor` |
| Format code | `make fmt` |
| Run go vet | `make vet` |
| Run linter | `make lint` |
| Run goimports | `make goimports` |
| Build container images | `make images` |

`make test` downloads envtest kubebuilder assets to `bin/k8s/` and runs Ginkgo with `--randomize-all --randomize-suites --race --trace --timeout=10m`. Tests use `controller-runtime/pkg/client/fake` for Kubernetes API simulation.

## Verification

After finishing any change, run these commands to verify correctness:

1. `make build` — Confirm the code compiles
2. `make test` — Run unit tests with race detection
3. `make generate` — If API types in `pkg/apis/` were modified, regenerate deepcopy and CRD files and verify no uncommitted diff remains
4. `make vendor` — If `go.mod` was modified, re-vendor dependencies
5. `make fmt` — Ensure code formatting is correct
6. `make vet` — Run static analysis

The minimum verification for any change is `make build && make test`. For API type changes, always run the full `make generate` which includes `make gen-deepcopy`, `make gen-crd`, `make goimports`, and `hack/verify-diff.sh`.

## Code Conventions

### Constants Over Inline Strings

Always define named constants for annotation keys, label names, resource names, flag names, default config values, and reason strings. Never use inline string literals for values that are referenced in more than one place or that represent a contract (annotations, labels, API field names).

### Methods vs Functions

Use **methods** (receiver functions) when the function accesses struct fields — clients, configs, embedded data, internal state. All reconciler operations, validator logic, status reporter operations, and MachineTarget operations are methods.

Use **standalone functions** when all inputs come from parameters. This includes:
- Constructors: `NewReconciler()`, `NewValidator()`, `NewStatusReporter()`, `NewConfig()`
- Pure transformations: `AutoscalerArgs()`, `ScaleDownArgs()`, `FilterString()`
- Simple helpers: `makePort()`, `objectReference()`

### Import Ordering

Two groups separated by a blank line:

1. **Standard library** (`context`, `fmt`, `errors`, `strings`, `testing`, etc.)
2. **Everything else** — external libraries and project-internal packages together, sorted alphabetically

There is no separate third group for project-internal imports; `github.com/openshift/cluster-autoscaler-operator/...` imports sit alongside other external imports. Import aliases use short descriptive names (e.g., `configv1`, `autoscalingv1`, `appsv1`, `apierrors`).

### Test Conventions

**Always write tests for new code.** Every code change must have unit test coverage.

**Prefer adding cases to existing test files** over creating new test files. Each source file has a corresponding `_test.go` file (e.g., `clusterautoscaler.go` / `clusterautoscaler_test.go`). Add new test cases to the existing table-driven tests when the new behavior fits an existing test function. Only create a new test function or file when the behavior being tested is genuinely different from existing tests.

**Table-driven tests with subtests** are the dominant pattern:

```go
testCases := []struct {
    label    string
    // inputs...
    // expected...
}{
    {
        label: "descriptive lowercase sentence",
        // ...
    },
}

for _, tc := range testCases {
    t.Run(tc.label, func(t *testing.T) {
        // test body
    })
}
```

Conventions within tests:
- Table variable is named `testCases` (or `testConfigs` in some files)
- Loop variable is `tc`
- Subtest labels are descriptive lowercase sentences (e.g., `"machine-api available"`, `"deployment wrong version"`)
- Test function names follow `TestFeatureName` (e.g., `TestReconcile`, `TestValidate`, `TestSetLimits`)
- Test helper functions and fixtures are defined at the top of the test file (e.g., `NewClusterAutoscaler()`, `newFakeReconciler()`)
- Test-specific constants are defined in the test file's own `const` block (e.g., `TestNamespace`, `TestCloudProvider`)
- Shared test helpers (builder-pattern wrappers) live in `test/helpers/helpers.go`
- Tests are in the same package as the source (no `_test` suffix), giving access to unexported identifiers
- Assertions use standard library `t.Errorf`/`t.Fatalf` with `got %v, want %v` format or `testify/assert`

### Error Handling

- **Named sentinel errors**: Use `var Err... = errors.New(...)` in `var` blocks for errors that callers compare against. These are exported (e.g., `ErrUnsupportedTarget`, `ErrTargetMissingAnnotations`).
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` to wrap errors with context.
- **Validation errors**: Use `utilerrors.NewAggregate()` from `k8s.io/apimachinery/pkg/util/errors` to collect multiple validation errors.
- **Error joining**: Use `errors.Join(err, e)` when accumulating errors from multiple operations.

### Logging

All logging uses `klog/v2` with printf-style formatting:

- `klog.Infof()` / `klog.Info()` — standard informational messages
- `klog.Errorf()` — errors that are handled/returned (not fatal)
- `klog.Warningf()` — non-critical warnings
- `klog.Fatalf()` — only in `main()` for unrecoverable startup errors
- `klog.V(2).Infof()` — detail-level debug messages
- `klog.V(4).Infof()` — very verbose output

## Common Pitfalls

1. **The ClusterAutoscaler CR must be named `"default"`.** CRs with any other name are silently ignored by the operator (configurable via `CLUSTER_AUTOSCALER_NAME` env var, but almost always `"default"` in practice).

2. **Do not hand-edit generated files.** Files matching `**/zz_generated.deepcopy.go` and CRD YAMLs in `install/` (`01_clusterautoscaler.crd.yaml`, `02_machineautoscaler.crd.yaml`, `11_provisioningrequest.crd.yaml`) are generated. Run `make generate` instead.

3. **Do not modify `vendor/` directly.** Run `make vendor` (which runs `go mod tidy && go mod vendor && go mod verify`). Commit vendor changes separately from logic changes.

4. **Install manifests live in `install/`, not `config/`.** The `config/` directory is empty. This project does not use kustomize.

5. **E2E tests live in a separate repository.** E2E tests are in [openshift/cluster-api-actuator-pkg](https://github.com/openshift/cluster-api-actuator-pkg), not here. Only unit tests live in this repo.

6. **The `go.mod` has a `replace` directive for autoscaler APIs.** `k8s.io/autoscaler/cluster-autoscaler/apis` uses a manually constructed pseudo-version to avoid pulling in kubernetes dependencies from the autoscaler module tree. Do not remove this replace.

7. **Install manifest numbering matters.** Files in `install/` are numbered for CVO ordering (CRDs first, then RBAC, then Deployment). Renumbering or adding files out of sequence can break cluster upgrades.

8. **`ENVTEST_K8S_VERSION` must match the k8s dependency version.** If you bump k8s dependencies, also update `ENVTEST_K8S_VERSION` in the Makefile.

## Human-in-the-Loop Triggers

Stop and consult a human before:

- **Modifying CRD API types** (`pkg/apis/`) — Require `make generate` and backward compatibility review
- **Changing install manifests** (`install/`) — Affect CVO-managed cluster upgrades; especially RBAC changes in `03_rbac.yaml` require security review
- **Affecting ClusterOperator status reporting** (`pkg/operator/status.go`) — Can impact cluster upgrade gates
- **Bumping dependencies** (`go.mod`) — May need coordinated bumps across k8s/controller-runtime/openshift deps; require `make vendor` as a separate commit
- **Changing Prometheus alerts** (`monitoring.go`) — Require coordination with monitoring team and documentation updates in `docs/user/alerts.md`
- **Modifying webhook validation logic** — Webhook failure policy is `Ignore`, so bugs silently allow invalid CRs

## Paired Changes

| If you change... | Also update... | Command |
|-----------------|----------------|---------|
| API types in `pkg/apis/` | Regenerate deepcopy + CRDs | `make generate` |
| `go.mod` dependencies | Re-vendor (separate commit) | `make vendor` |
| Autoscaler CLI args in `clusterautoscaler.go` | Tests in `clusterautoscaler_test.go` | `make test` |
| Prometheus alerts in `monitoring.go` | Alert docs in `docs/user/alerts.md` | manual |
| RBAC requirements | `install/03_rbac.yaml` | manual |
| New env var in `config.go` | `README.md` and `install/07_deployment.yaml` if needed | manual |

## Commit Convention

[Conventional Commits](https://www.conventionalcommits.org/) with optional scope:

```
feat(autoscaler): add support for startupTaints
fix(machineautoscaler): correct nil pointer in target lookup
chore: bump to go 1.26
docs: add contributing guidelines
test: add validator edge case coverage
refactor: simplify status reporter polling
```

Supported prefixes: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`.

Vendor changes should be committed separately from logic changes.

## Review Policy

Every change must be understood and approved by two humans. The PR author counts as one if they fully understand the code. AI-assisted PRs where the author does not fully understand the code require two reviewer approvals. PRs from deterministic CI automation can merge with a single human review.

## Further Reading

- [CONTRIBUTING.md](CONTRIBUTING.md) — Contribution workflow, PR commands, test expectations, code style
- [README.md](README.md) — Project overview and development setup
- [docs/dev/metrics.md](docs/dev/metrics.md) — Prometheus metrics scraping guide
- [docs/user/alerts.md](docs/user/alerts.md) — Autoscaler alert documentation
- [examples/](examples/) — Sample ClusterAutoscaler and MachineAutoscaler CRs
- Upstream autoscaler: [kubernetes-sigs/cluster-autoscaler](https://github.com/kubernetes-sigs/cluster-autoscaler)
- E2E tests: [openshift/cluster-api-actuator-pkg](https://github.com/openshift/cluster-api-actuator-pkg)
- Machine API Operator: [openshift/machine-api-operator](https://github.com/openshift/machine-api-operator)
