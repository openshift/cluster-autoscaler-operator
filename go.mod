module github.com/openshift/cluster-autoscaler-operator

go 1.13

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.38.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/openshift/api v0.0.0-20200916161728-83f0cb093902
	github.com/openshift/client-go v0.0.0-20200827190008-3062137373b5
	github.com/openshift/library-go v0.0.0-20200917093739-70fa806b210a
	github.com/stretchr/testify v1.5.1
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.3.0
)

// to replace github.com/coreos/prometheus-operator v0.38.0 dep
replace k8s.io/client-go => k8s.io/client-go v0.19.0
