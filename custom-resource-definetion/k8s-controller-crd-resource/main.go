package main

import (
	"flag"
	etcdClientSetVersion "github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/client/clientset/versioned"
	etcdInformerVersion "github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/client/informers/externalversions"
	"github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/signals"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/klog/v2"
	"time"
)

var (
	masterUrl         = ""
	kubeConfigPath    = ""
	maxGoroutineCount = 2
)

func main() {

	stop := signals.SetupSignalHandler()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		kubeConfig, err = clientcmd.BuildConfigFromFlags(masterUrl, kubeConfigPath)
		if err != nil {
			klog.Fatalf("local config err: %+v", err)
		}
	}

	kubeClientSet, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("new clientSet err: %+v", err)
	}

	etcdClientSet, err := etcdClientSetVersion.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("new etcd clientSet err: %+v", err)
	}

	etcdSharedInformers := etcdInformerVersion.NewSharedInformerFactory(etcdClientSet, time.Second*30)

	controller := NewController(kubeClientSet, etcdClientSet, etcdSharedInformers.Crd().V1alpha1().Etcds())

	go etcdSharedInformers.Start(stop)

	if err = controller.Run(maxGoroutineCount, stop); err != nil {
		klog.Fatalf("error running controller: %s", err)
	}
}

func init() {
	flag.StringVar(&kubeConfigPath, "kube-config", "", "usage kube-config file path")
	flag.StringVar(&masterUrl, "master-url", "", "usage master-url")
	flag.Parse()
}
