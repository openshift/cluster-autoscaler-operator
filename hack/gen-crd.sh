#!/usr/bin/env bash

set -eu

go run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen crd --domain openshift.io

echo "Copying generated CRDs"
cp config/crds/autoscaling_v1_clusterautoscaler.yaml install/01_clusterautoscaler.crd.yaml
cp config/crds/autoscaling_v1beta1_machineautoscaler.yaml install/02_machineautoscaler.crd.yaml
