module github.com/openshift/cluster-autoscaler-operator

go 1.15

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.29.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20210412212256-79bd8cfbbd59
	github.com/openshift/client-go v0.0.0-20210409155308-a8e62c60e930
	github.com/openshift/library-go v0.0.0-20210408164723-7a65fdb398e2
	github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	k8s.io/api v0.21.0-rc.0
	k8s.io/apimachinery v0.21.0-rc.0
	k8s.io/client-go v0.21.0-rc.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.9.0-alpha.1
	sigs.k8s.io/controller-tools v0.3.0
)
