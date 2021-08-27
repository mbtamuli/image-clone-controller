package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=list;watch;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;get;patch;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deployment", req.NamespacedName)
	log.Info("Reconciling...")

	if strings.Contains(os.Getenv("EXCLUDED_NAMESPACES"), req.Namespace) {
		log.Info("Skipping namespace from the list of EXCLUDED_NAMESPACES")
		return ctrl.Result{}, nil
	}

	registry := os.Getenv("REGISTRY")
	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	err := RegistryLogin(registry, registryUsername, registryPassword)
	if err != nil {
		log.Error(err, "Failed to log into registry")
		return ctrl.Result{}, nil
	}

	// Fetch the Deployment instance
	deployment := &appsv1.Deployment{}
	err = r.Client.Get(ctx, req.NamespacedName, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Check if the secret already exists, if not create a new one
	secret := &corev1.Secret{}
	err = r.Client.Get(ctx, req.NamespacedName, secret)
	if err != nil && errors.IsNotFound(err) {
		secret, err := newDockerCfgSecret(req.Namespace, req.Name, registry, registryUsername, registryPassword)
		if err != nil {
			log.Error(err, "Failed to create secret object")
			return ctrl.Result{}, err
		}
		log.Info("Creating a Secret for deployment imagePullSecret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		if err := r.Create(ctx, secret); err != nil {
			return ctrl.Result{}, err
		}
		// Secret created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		return ctrl.Result{}, err
	}

	deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: secret.Name}}

	containers := deployment.Spec.Template.Spec.Containers
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(registry, image) {
			log.Info("Starting image backup", "image", fmt.Sprintf("%s/%s/%s", registry, registryUsername, image))
			newImage, err := ImageBackup(registry, registryUsername, image)
			if err != nil {
				log.Error(err, "Unable to backup image")
				return ctrl.Result{}, err
			}
			log.Info("Replacing image", fmt.Sprintf("%s/%s/%s", registry, registryUsername, image), newImage)
			deployment.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	err = r.Update(ctx, deployment)
	if err != nil {
		log.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}
