package controller

import (
	"github.com/openshift/cluster-autoscaler-operator/pkg/operator"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, *operator.Config) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, c *operator.Config) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, c); err != nil {
			return err
		}
	}
	return nil
}
