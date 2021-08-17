module github.com/openshift/cluster-autoscaler-operator

go 1.16

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/openshift/api v0.0.0-20210816181336-8ff39b776da3
	github.com/openshift/client-go v0.0.0-20210730113412-1811c1b3fc0e
	github.com/openshift/library-go v0.0.0-20210811133500-5e31383de2a7
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/prometheus/common v0.29.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/atomic v1.8.0 // indirect
	golang.org/x/net v0.0.0-20210610132358-84b48f89b13b // indirect
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/controller-runtime v0.9.3
	sigs.k8s.io/controller-tools v0.6.2
)

replace k8s.io/client-go => k8s.io/client-go v0.22.0 // Required because prometheus operator has a wrong version string
