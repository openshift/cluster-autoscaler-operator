module github.com/openshift/cluster-autoscaler-operator

go 1.15

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/prometheus-operator v0.29.0
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/openshift/api v0.0.0-20200916161728-83f0cb093902
	github.com/openshift/client-go v0.0.0-20200827190008-3062137373b5
	github.com/openshift/library-go v0.0.0-20200917093739-70fa806b210a
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/stretchr/testify v1.5.1
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/net v0.0.0-20201031054903-ff519b6c9102 // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	golang.org/x/sys v0.0.0-20201101102859-da207088b7d1 // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver v0.19.3 // indirect
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20200923155610-8b5066479488 // indirect
	k8s.io/utils v0.0.0-20201027101359-01387209bb0d
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/controller-tools v0.3.0
	sigs.k8s.io/structured-merge-diff/v4 v4.0.2 // indirect
)

// to replace github.com/coreos/prometheus-operator v0.38.0 dep
replace k8s.io/client-go => k8s.io/client-go v0.19.0
