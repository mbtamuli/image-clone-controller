package main

import (
	"flag"
	"fmt"
	// kubeinformers "k8s.io/client-go/informers"
	// "k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/clientcmd"
)

func main() {
	// kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
	// namespace := flag.String("exclude-namespace", "kube-system", "skip watching resources in this namespace")
	registry := flag.String("registry", "", "Registry to use (defaults to DockerHub)")
	registryUsername := flag.String("registry-username", "", "Username for registry login")
	registryPassword := flag.String("registry-password", "", "Password for registry login")

	flag.Parse()

	if err := RegistryLogin(*registry, *registryUsername, *registryPassword); err != nil {
		fmt.Printf("unable to log in: %s\n", err)
	}

	if err := ImageBackup("nginx:1.19"); err != nil {
		fmt.Printf("unable to backup image: %s\n", err)
	}
	// stopCh := make(chan struct{})

	// // use the current context in kubeconfig
	// config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// // create the clientset
	// clientset, err := kubernetes.NewForConfig(config)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, time.Second*30)

	// controller := NewController(clientset,
	// 	kubeInformerFactory.Apps().V1().Deployments(),
	// 	*namespace)

	// kubeInformerFactory.Start(stopCh)

	// if err = controller.Run(stopCh); err != nil {
	// 	fmt.Printf("Error running controller: %s", err.Error())
	// }

}
