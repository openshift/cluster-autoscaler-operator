module github.com/openshift/cluster-autoscaler-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.29.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20210816181336-8ff39b776da3
	github.com/openshift/client-go v0.0.0-20210730113412-1811c1b3fc0e
	github.com/openshift/library-go v0.0.0-20210811133500-5e31383de2a7
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/controller-runtime v0.9.3
	sigs.k8s.io/controller-tools v0.6.2
)
