package main

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/openshift/cluster-autoscaler-operator/pkg/apis"
	v1 "k8s.io/api/core/v1"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	rest "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	namespace = "openshift-cluster-api"
	caName    = "default"
)

var F *Framework

type Framework struct {
	Client     client.Client
	RESTClient *rest.RESTClient
}

func newClient() error {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := client.New(cfg, client.Options{})
	if err != nil {
		return err
	}

	configShallowCopy := *cfg
	gv := v1.SchemeGroupVersion
	configShallowCopy.GroupVersion = &gv
	configShallowCopy.APIPath = "/api"
	configShallowCopy.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	rc, err := rest.RESTClientFor(&configShallowCopy)
	if err != nil {
		return fmt.Errorf("unable to build rest client: %v", err)
	}

	F = &Framework{Client: client, RESTClient: rc}

	return nil
}

func main() {
	flag.Parse()

	if err := apis.AddToScheme(scheme.Scheme); err != nil {
		glog.Fatal(err)
	}

	if err := newClient(); err != nil {
		glog.Fatal(err)
	}

	if err := runSuite(); err != nil {
		glog.Fatal(err)
	}
}

func runSuite() error {
	if err := ExpectOperatorAvailable(); err != nil {
		glog.Errorf("FAIL: ExpectOperatorAvailable: %v", err)
		return err
	}
	glog.Info("PASS: ExpectOperatorAvailable")

	if err := CreateClusterAutoscaler(); err != nil {
		glog.Errorf("FAIL: CreateClusterAutoscaler: %v", err)
		return err
	}
	glog.Info("PASS: CreateClusterAutoscaler")

	if err := ExpectClusterAutoscalerAvailable(); err != nil {
		glog.Errorf("FAIL: ExpectClusterAutoscalerAvailable: %v", err)
		return err
	}
	glog.Info("PASS: ExpectClusterAutoscalerAvailable")

	return nil
}
