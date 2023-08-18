package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	endpointcontroller "github.com/hydra-cni/hydra/pkg/controller/endpoint"
	endpointslicecontroller "github.com/hydra-cni/hydra/pkg/controller/endpointslice"
)

var kubeconfig string

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
}

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()

	if err != nil {
		// fallback to kube config
		if val := os.Getenv("KUBECONFIG"); len(val) != 0 {
			kubeconfig = val
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			fmt.Printf("The kubeconfig cannot be loaded: %v\n", err)
			os.Exit(1)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, time.Hour*24)

	stop := make(chan struct{})
	defer close(stop)
	factory.Start(stop)

	ctx := context.Background()
	ep := endpointcontroller.NewEndpointController(
		factory.Core().V1().Pods(),
		factory.Core().V1().Services(),
		factory.Core().V1().Endpoints(),
		clientset,
		1*time.Second,
	)
	eps := endpointslicecontroller.NewController(ctx,
		factory.Core().V1().Pods(),
		factory.Core().V1().Services(),
		factory.Core().V1().Nodes(),
		factory.Discovery().V1().EndpointSlices(),
		100,
		clientset,
		1*time.Second)

	factory.Start(wait.NeverStop)

	go ep.Run(ctx, 1)
	go eps.Run(ctx, 1)

	select {}
}
