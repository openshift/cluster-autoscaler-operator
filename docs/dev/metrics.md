# CAO Metrics

The Cluster Autoscaler Operator(CAO) and the Cluster Autoscaler(CA) use the
[Prometheus project](https://prometheus.io/) to expose metrics. To consume
these metrics directly from the CAO or the CA you will need to perform
HTTP GET requests to a specific port and URI on each application(CAO/CA). The
URI for all metrics is `/metrics`, see the Prometheus documentation for query
parameter options. To find the exposed metrics port for the CAO you can either
inspect the Deployment resource or the
[install manifest](https://github.com/openshift/cluster-autoscaler-operator/blob/master/install/07_deployment.yaml)
to find the environment variable `METRICS_PORT`, the default value for this is `9191`.
Finding the metrics port for the CA is slightly more complex, at the time of this
writing there is no explicitly mentioned
service for metrics on the CA so you should use the default port of `8085`.

**Example CAO metrics scrape procedure**
1. Forward the metrics port from the CAO to a local port
   ```
   $ oc port-forward -n openshift-machine-api deployment/cluster-autoscaler-operator 9191:9191
   ```
2. Perform an HTTP GET request on the local port
   ```
   $ curl http://localhost:9191/metrics
   ```

**Example CA metrics scrape procedure**
1. Forward the metrics port from the CA to a local port
   ```
   $ oc port-forward -n openshift-machine-api deployment/cluster-autoscaler-default 8085:8085
   ```
2. Perform an HTTP GET request on the local port
   ```
   $ curl http://localhost:8085/metrics
   ```

The Cluster Autoscaler Operator reports the following metrics:

## Metrics provided by the controller runtime

The [controller runtime](https://github.com/kubernetes-sigs/controller-runtime)
integration with the operator provides several metrics about the internals of the
operator. You can find more information about these metrics names and their labels through
the following links:

### Kubernetes controller metrics

These metric names begin with `controller_runtime_reconcile_`. The labels
`controller="cluster_autoscaler_controller"` and
`controller="machine_autoscaler_controller"` can be used to refine queries against these metrics.
* [Controller runtime reconciliation metrics implementation](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/internal/controller/metrics/metrics.go)

### Admission webhook metrics

These metric names begin with `controller_runtime_webhook_`.  The label
`webhook="/validate-clusterautoscalers"` can be used to refine the queries for these metrics.
* [Controller runtime webhook metrics implementation](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/webhook/internal/metrics/metrics.go)

### API REST server metrics

These metric names begin with `rest_client_` and `reflector_`. The labels
`url` and `verb` can be used to refine these metrics further. The `url` label
should contain the address to the Kubernetes API server, while `verb`
containts the type of HTTP request (eg `"GET"`).
* [Controller runtime Prometheus REST server metrics implementation](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/metrics/client_go_adapter.go)

### Prometheus work queue metrics

These metric names begin with `workqueue_`. The labels
`name="cluster_autoscaler_controller"` and `name="machine_autoscaler_controller"`
can be used to refine queries against these metrics.
* [Controller runtime Prometheus work queue metrics implementation](https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/metrics/workqueue.go)

## Metrics about the Prometheus collectors

Prometheus provides some default metrics about the internal state
of the running process and the metric collection. You can find more information
about these metric names and their labels through the following links:

* [Prometheus documentation, Standard and runtime collectors](https://prometheus.io/docs/instrumenting/writing_clientlibs/#standard-and-runtime-collectors)
* [Prometheus client Go language collectors](https://github.com/prometheus/client_golang/blob/master/prometheus/go_collector.go)

# Cluster Autoscaler Metrics

The Cluster Autoscaler Operator is responsible for lifecycle management of the
[Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler) on OpenShift. The metrics
described previous in this document are specifically from that operator. If you would
like to gather metrics from the cluster autoscaler itself please see the
[Cluster Autoscaler Monitoring](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/proposals/metrics.md)
documentation.
