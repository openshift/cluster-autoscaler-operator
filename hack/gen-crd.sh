#!/usr/bin/env bash

set -eu

echo "Building controller-gen tool..."
go build -o bin/controller-gen github.com/openshift/cluster-autoscaler-operator/vendor/sigs.k8s.io/controller-tools/cmd/controller-gen

bin/controller-gen crd --domain openshift.io

echo "Copying generated CRDs"
cp config/crds/autoscaling_v1_clusterautoscaler.yaml install/01_clusterautoscaler.crd.yaml
cp config/crds/autoscaling_v1beta1_machineautoscaler.yaml install/02_machineautoscaler.crd.yaml
