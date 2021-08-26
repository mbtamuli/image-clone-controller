package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
	namespace := flag.String("exclude-namespaces", "kube-system", "skip watching resources in the list of comma separated namespaces")
	repository := flag.String("repository", "mbtamuli", "Repository to use. For example, will default to 'mbtamuli', so the image will be pushed to REGISTRY/mbtamuli/IMAGE:TAG")
	registry := flag.String("registry", "", "Registry to use (defaults to DockerHub)")
	registryUsername := flag.String("registry-username", "", "Username for registry login")
	registryPassword := flag.String("registry-password", "", "Password for registry login")

	flag.Parse()

	zapLogger, _ := zap.NewProduction()
	defer zapLogger.Sync() // flushes buffer, if any
	logger := zapLogger.Sugar()

	stopCh := make(chan struct{})

	clientset, err := getClient(*kubeconfig)
	if err != nil {
		logger.Fatal(err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, time.Second*30)

	controller := NewController(clientset,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Apps().V1().DaemonSets(),
		*namespace,
		*registry,
		*registryUsername,
		*registryPassword,
		*repository,
		logger)

	kubeInformerFactory.Start(stopCh)

	logger.Infof("Logging into registry: %s", *registry)
	err = RegistryLogin(*registry, *registryUsername, *registryPassword)
	if err != nil {
		logger.Fatal("unable to login to registry: %s", err)
	}

	logger.Infof("Starting the controller")
	if err = controller.Run(stopCh); err != nil {
		logger.Fatal("error running controller: %s", err)
	}

}

func getClient(kubeconfig string) (*kubernetes.Clientset, error) {
	var kubeClient *kubernetes.Clientset
	if inClusterConfig, err := rest.InClusterConfig(); err != nil {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("unable to build config from kubeconfig file: %s", err)
		}
		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("unable to build clientset from config: %s", err)
		}
	} else {
		kubeClient, err = kubernetes.NewForConfig(inClusterConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to build clientset from in cluster config: %s", err)
		}
	}

	return kubeClient, nil
}
