#!/usr/bin/env bash

set -eu

function annotate_crd() {
  script1='/^  annotations:/a\
\ \ \ \ exclude.release.openshift.io/internal-openshift-hosted: "true"\
\ \ \ \ include.release.openshift.io/self-managed-high-availability: "true"\
\ \ \ \ capability.openshift.io/name: MachineAPI\
\ \ \ \ include.release.openshift.io/single-node-developer: "true"'
  script2='/^    controller-gen.kubebuilder.io\/version: .*$/d'
  input="${1}"
  output="${2}"
  sed -e "${script1}" -e "${script2}" "${input}" > "${output}"
}

# TODO elmiko, remove this function once ProvisioningRequest is no longer behind a feature gate
function annotate_provreq_crd() {
  script1='/^  annotations:/a\
\ \ \ \ feature-gate.release.openshift.io/ProvisioningRequestAvailable: "true"\
\ \ \ \ release.openshift.io/feature-set: DevPreviewNoUpgrade\
\ \ \ \ exclude.release.openshift.io/internal-openshift-hosted: "true"\
\ \ \ \ include.release.openshift.io/self-managed-high-availability: "true"\
\ \ \ \ capability.openshift.io/name: MachineAPI\
\ \ \ \ include.release.openshift.io/single-node-developer: "true"'
  script2='/^    controller-gen.kubebuilder.io\/version: .*$/d'
  input="${1}"
  output="${2}"
  sed -e "${script1}" -e "${script2}" "${input}" > "${output}"
}

go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1 paths=./pkg/apis/...
go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1 paths=./vendor/k8s.io/autoscaler/cluster-autoscaler/apis/provisioningrequest/autoscaling.x-k8s.io/v1/...

echo "Copying generated CRDs"
annotate_crd config/crd/autoscaling.openshift.io_clusterautoscalers.yaml install/01_clusterautoscaler.crd.yaml
annotate_crd config/crd/autoscaling.openshift.io_machineautoscalers.yaml install/02_machineautoscaler.crd.yaml
# TODO elmiko, change this to annotate_crd once ProvisioningRequest is no longer behind a feature gate
annotate_provreq_crd config/crd/autoscaling.x-k8s.io_provisioningrequests.yaml install/11_provisioningrequest.crd.yaml
rm -rf ./config/crd
