package controller

import (
	"github.com/openshift/cluster-autoscaler-operator/pkg/controller/clusterautoscaler"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, clusterautoscaler.Add)
}
