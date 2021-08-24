package main

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
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
	workqueue         workqueue.RateLimitingInterface
	namespace         string
	registry          string
	repository        string
}

func NewController(
	clientset kubernetes.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	namespace,
	registry,
	repository string) *Controller {

	controller := &Controller{
		kubeclientset:     clientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "image-clone-controller"),
		namespace:         namespace,
		registry:          registry,
		repository:        repository,
	}

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueDeployment,
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
		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		fmt.Printf("Successfully synced '%s'\n", key)
		return nil
	}(obj)

	if err != nil {
		fmt.Println(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncHandler(key string) error {
	fmt.Printf("Starting handler for %s\n", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Println(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Println(fmt.Errorf("deployment '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	deploymentImage := deployment.Spec.Template.Spec.Containers[0].Image

	_, err = ImageBackup(c.registry, c.repository, deploymentImage)
	if err != nil {
		return fmt.Errorf("unable to backup image: %s", err)
	}

	return nil
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
	if !strings.Contains(c.namespace, namespace) {
		c.workqueue.Add(key)
	}
}
