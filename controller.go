package main

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	kubeclientset     kubernetes.Interface
	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	daemonsetsLister  appslisters.DaemonSetLister
	daemonsetsSynced  cache.InformerSynced
	workqueue         workqueue.RateLimitingInterface
	namespace         string
	registry          string
	registryUsername  string
	registryPassword  string
	repository        string
}

func NewController(
	clientset kubernetes.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	daemonsetInformer appsinformers.DaemonSetInformer,
	namespace,
	registry,
	registryUsername,
	registryPassword,
	repository string) *Controller {

	controller := &Controller{
		kubeclientset:     clientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		daemonsetsLister:  daemonsetInformer.Lister(),
		daemonsetsSynced:  daemonsetInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "image-clone-controller"),
		namespace:         namespace,
		registry:          registry,
		registryUsername:  registryUsername,
		registryPassword:  registryPassword,
		repository:        repository,
	}

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueDeployment,
	})

	daemonsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueDaemonset,
	})

	return controller
}

// Run will set up the syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer c.workqueue.ShutDown()

	fmt.Println("Starting image-clone-controller")

	fmt.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	fmt.Println("Starting workers")
	go wait.Until(c.runWorker, time.Second, stopCh)

	fmt.Println("Started workers")
	<-stopCh
	fmt.Println("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			fmt.Println(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		splitKey := strings.SplitN(key, "/", 2)
		objectType := splitKey[0]
		objectKey := splitKey[1]

		if objectType == "deployment" {
			if err := c.deploymentSyncHandler(objectKey); err != nil {
				c.workqueue.AddRateLimited(objectKey)
				return fmt.Errorf("error syncing '%s': %s, requeuing", objectKey, err.Error())
			}
		}
		if objectType == "daemonset" {
			if err := c.daemonsetSyncHandler(objectKey); err != nil {
				c.workqueue.AddRateLimited(objectKey)
				return fmt.Errorf("error syncing '%s': %s, requeuing", objectKey, err.Error())
			}
		}

		c.workqueue.Forget(obj)
		fmt.Printf("Successfully synced '%s'\n", objectKey)
		return nil
	}(obj)

	if err != nil {
		fmt.Println(err)
		return true
	}

	return true
}

// enqueueDeployment takes a Deployment resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (c *Controller) enqueDeployment(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		fmt.Println(err)
	}
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Println(fmt.Errorf("invalid resource key: %s", key))
	}
	key = "deployment/" + key
	if !strings.Contains(c.namespace, namespace) {
		c.workqueue.Add(key)
	}
}

// enqueueDaemonset takes a Daemonset resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (c *Controller) enqueDaemonset(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		fmt.Println(err)
	}
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Println(fmt.Errorf("invalid resource key: %s", key))
	}
	key = "daemonset/" + key
	if !strings.Contains(c.namespace, namespace) {
		c.workqueue.Add(key)
	}
}
