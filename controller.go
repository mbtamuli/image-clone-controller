package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
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
	registryUsername  string
	registryPassword  string
	repository        string
}

// DockerConfigJSON represents a local docker auth config file
// for pulling images.
type DockerConfigJSON struct {
	Auths DockerConfig `json:"auths" datapolicy:"token"`
	// +optional
	HttpHeaders map[string]string `json:"HttpHeaders,omitempty" datapolicy:"token"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

// DockerConfigEntry holds the user information that grant the access to docker registry
type DockerConfigEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty" datapolicy:"password"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty" datapolicy:"token"`
}

func NewController(
	clientset kubernetes.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	namespace,
	registry,
	registryUsername,
	registryPassword,
	repository string) *Controller {

	controller := &Controller{
		kubeclientset:     clientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
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
		fmt.Printf("invalid resource key: %s\n", key)
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("deployment '%s' in work queue no longer exists\n", key)
			return nil
		}
		return err
	}

	secretApplyConfig := applycorev1.Secret(name, namespace)
	dockerConfigJSONContent, err := handleDockerCfgJSONContent(c.registryPassword, c.registryPassword, c.registry)
	if err != nil {
		return err
	}
	if secretApplyConfig.Data == nil {
		secretApplyConfig.Data = make(map[string][]byte)
	}
	secretApplyConfig.Data[corev1.DockerConfigJsonKey] = dockerConfigJSONContent
	dockerSecret, err := c.kubeclientset.CoreV1().Secrets(namespace).Apply(context.TODO(), secretApplyConfig, metav1.ApplyOptions{FieldManager: "image-clone-controller"})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: dockerSecret.Name}}

	containers := deployment.Spec.Template.Spec.Containers
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(c.repository, image) {
			newImage, err := ImageBackup(c.registry, c.repository, image)
			if err != nil {
				return fmt.Errorf("unable to backup image: %s", err)
			}

			deployment.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	_, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
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

// handleDockerCfgJSONContent and encodeDockerConfigFieldAuth copied from
// https://github.com/kubernetes/kubectl/blob/723266d1458429c5741ec6c0b5c315e72ec6f7cb/pkg/cmd/create/create_secret_docker.go#L289-L308

// handleDockerCfgJSONContent serializes a ~/.docker/config.json file
func handleDockerCfgJSONContent(username, password, server string) ([]byte, error) {
	dockerConfigAuth := DockerConfigEntry{
		Username: username,
		Password: password,
		Auth:     encodeDockerConfigFieldAuth(username, password),
	}
	dockerConfigJSON := DockerConfigJSON{
		Auths: map[string]DockerConfigEntry{server: dockerConfigAuth},
	}

	return json.Marshal(dockerConfigJSON)
}

// encodeDockerConfigFieldAuth returns base64 encoding of the username and password string
func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
