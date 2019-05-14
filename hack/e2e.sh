#!/bin/bash

set -euo pipefail

GOPATH="$(mktemp -d)"
export GOPATH

ACTUATOR_PKG="github.com/openshift/cluster-api-actuator-pkg"

go get -u -d "${ACTUATOR_PKG}/..."

go test -timeout 60m \
   -v "${ACTUATOR_PKG}/pkg/e2e" \
   -kubeconfig "${KUBECONFIG:-${HOME}/.kube/config}" \
   -machine-api-namespace "${NAMESPACE:-openshift-machine-api}" \
   -ginkgo.v \
   -ginkgo.noColor=true \
   -args -v 5 -logtostderr true
