#!/usr/bin/env bash

set -eu

function annotate_crd() {
  script='/^metadata:/a\
\ \ annotations:\
\ \ \ \ exclude.release.openshift.io/internal-opernshift-hosted: "true"'
  input="${1}"
  output="${2}"
  sed -e "${script}" "${input}" > "${output}"
}

go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd paths=./pkg/apis/...

echo "Copying generated CRDs"
annotate_crd config/crd/autoscaling.openshift.io_clusterautoscalers.yaml install/01_clusterautoscaler.crd.yaml
annotate_crd config/crd/autoscaling.openshift.io_machineautoscalers.yaml install/02_machineautoscaler.crd.yaml
rm -rf ./config/crd
