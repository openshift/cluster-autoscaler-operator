# Cluster Autoscaler Operator

The cluster-autoscaler-operator manages deployments of the OpenShift
[Cluster Autoscaler][1] using the [cluster-api][2] provider.

[1]: https://github.com/openshift/kubernetes-autoscaler/tree/master/cluster-autoscaler
[2]: https://github.com/kubernetes-sigs/cluster-api


## Custom Resource Definitions

The operator manages the following custom resources:

- __ClusterAutoscaler__: This is a singleton resource which controls the
  configuration of the cluster's autoscaler instance.  The operator will
  only respond to the ClusterAutoscaler resource named "default" in the
  managed namespace, i.e. the value of the `WATCH_NAMESPACE` environment
  variable.  ([Example][ClusterAutoscaler])

  The fields in the spec for ClusterAutoscaler resources correspond to
  command-line arguments to the cluster-autoscaler.  The example
  linked above results in the following invocation:

  ```
    Command:
      cluster-autoscaler
    Args:
      --logtostderr
      --balance-similar-node-groups=true
      --v=1
      --cloud-provider=clusterapi
      --namespace=openshift-machine-api
      --leader-elect-lease-duration=137s
      --leader-elect-renew-deadline=107s
      --leader-elect-retry-period=26s
      --expander=random
      --expendable-pods-priority-cutoff=-10
      --max-nodes-total=24
      --cores-total=8:128
      --memory-total=4:256
      --gpu-total=nvidia.com/gpu:0:16
      --gpu-total=amd.com/gpu:0:4
      --scale-down-enabled=true
      --scale-down-delay-after-add=10s
      --scale-down-delay-after-delete=10s
      --scale-down-delay-after-failure=10s
      --scale-down-utilization-threshold=0.4
      --ignore-daemonsets-utilization=false
      --skip-nodes-with-local-storage=true
  ```

- __MachineAutoscaler__: This resource targets a node group and manages
  the annotations to enable and configure autoscaling for that group,
  e.g. the min and max size, and GPU label.  Currently only `MachineSet` objects can be
  targeted.  ([Example][MachineAutoscaler])

[ClusterAutoscaler]: https://github.com/openshift/cluster-autoscaler-operator/blob/master/examples/clusterautoscaler.yaml
[MachineAutoscaler]: https://github.com/openshift/cluster-autoscaler-operator/blob/master/examples/machineautoscaler.yaml


## Development

```sh-session
## Build, Test, & Run
$ make build
$ make test

$ export WATCH_NAMESPACE=openshift-machine-api
$ ./bin/cluster-autoscaler-operator -alsologtostderr
```

The Cluster Autoscaler Operator is designed to be deployed on
OpenShift by the [Cluster Version Operator][CVO], but it's possible to
run it directly on any vanilla Kubernetes cluster that has the
[machine-api][machine-api] components available.  To do so, apply the
manifests in the install directory: `kubectl apply -f ./install`

This will create the `openshift-machine-api` namespace, register the
custom resource definitions, configure RBAC policies, and create a
deployment for the operator.

[CVO]: https://github.com/openshift/cluster-version-operator
[machine-api]: https://github.com/openshift/cluster-api
[cluster-api]: https://github.com/kubernetes-sigs/cluster-api


### End-to-End Tests

You can run the e2e test suite with `make test-e2e`.  These tests
assume the presence of a cluster already running the operator, and
that the `KUBECONFIG` environment variable points to a configuration
granting admin rights on said cluster.

If running make targets in container with podman and encountering permission issues, see [hacking-guide](https://github.com/openshift/machine-api-operator/blob/master/docs/dev/hacking-guide.md#troubleshooting-make-targets).


## Validating Webhooks

By default the operator starts an HTTP server for webhooks and
registers a `ValidatingWebhookConfiguration` with the API server for
both the `ClusterAutoscaler` and `MachineAutoscaler` types.  This can
be disabled via the `WEBHOOKS_ENABLED` environment variable.  At the
moment, reconciliation of the webhook configuration is only performed
once at startup after leader-election has succeeded.

If the webhook server is enabled, you must provide a TLS certificate
and key as well as a CA certificate to the operator.  The location of
these is controlled by the `WEBHOOKS_CERT_DIR` environment variable,
which defaults to: `/etc/cluster-autoscaler-operator/tls`

The files must be in the following locations:

  - `${WEBHOOKS_CERT_DIR}/tls.crt`
  - `${WEBHOOKS_CERT_DIR}/tls.key`
  - `${WEBHOOKS_CERT_DIR}/service-ca/ca-cert.pem`

The default cluster-autoscaler-operator deployment on OpenShift will
generate the TLS assets automatically with the help of the OpenShift
[service-ca-operator][service-ca-operator].  This works by annotating
the `Service` object associated with the operator, which causes the
service-ca-operator to generate a TLS certificate and inject it into a
`Secret`, which is then mounted into the operator pod.  Additionally,
the service-ca-operator injects its CA certificate into a `ConfigMap`,
which is also mounted.  The operator then uses the TLS certificate and
key to secure the webhook HTTP server, and injects the CA certificate
into the webhook configuration registered with the API server.

Updates to the TLS certificate and key are handled transparently.  The
[controller-runtime][controller-runtime] library the operator is based
on watches the files mounted in the pod for changes and updates HTTP
server's TLS configuration.  Updates to the CA certificate are not
handled automatically, however a restart of the operator will load the
new CA certificate and update the webhook configuration.  This is not
usually a problem in practice because CA certificates are generally
long-lived and the webhook configuration is set to ignore
communication failures as the validations are merely a convenience.

[service-ca-operator]: https://github.com/openshift/service-ca-operator
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
