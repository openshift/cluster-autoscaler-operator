#!/usr/bin/env bash

set -eu

function annotate_crd() {
  script1='/^  annotations:/a\
\ \ \ \ exclude.release.openshift.io/internal-openshift-hosted: "true"\
\ \ \ \ include.release.openshift.io/self-managed-high-availability: "true"'
  script2='/^    controller-gen.kubebuilder.io\/version: .*$/d'
  input="${1}"
  output="${2}"
  sed -e "${script1}" -e "${script2}" "${input}" > "${output}"
}

go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1 paths=./pkg/apis/...

echo "Copying generated CRDs"
annotate_crd config/crd/autoscaling.openshift.io_clusterautoscalers.yaml install/01_clusterautoscaler.crd.yaml
annotate_crd config/crd/autoscaling.openshift.io_machineautoscalers.yaml install/02_machineautoscaler.crd.yaml
rm -rf ./config/crd
