module github.com/openshift/cluster-autoscaler-operator

go 1.13

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.38.0
	github.com/openshift/api v0.0.0-20200331152225-585af27e34fd
	github.com/openshift/client-go v0.0.0-20200326155132-2a6cd50aedd0
	github.com/openshift/library-go v0.0.0-20200402123743-4015ba624cae
	github.com/stretchr/testify v1.4.0
	golang.org/x/text v0.3.3 // indirect
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200327001022-6496210b90e8
	sigs.k8s.io/controller-runtime v0.5.1-0.20200330174416-a11a908d91e0
	sigs.k8s.io/controller-tools v0.2.9-0.20200331153640-3c5446d407dd
)

// to replace github.com/coreos/prometheus-operator v0.38.0 dep
replace k8s.io/client-go => k8s.io/client-go v0.18.0
