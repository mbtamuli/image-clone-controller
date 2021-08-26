package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	ExcludeNamespaces string
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=list;watch;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if strings.Contains(r.ExcludeNamespaces, req.Namespace) {
		return ctrl.Result{}, nil
	}

	log := r.Log.WithValues("deployment", req.NamespacedName)

	// Check if the registry-secret exists
	registrySecret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "registry-cred", Namespace: "default"}, registrySecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("registry-cred secret not found: %v", err)
	}
	registry := string(registrySecret.Data["registry"])
	registryUsername := string(registrySecret.Data["registry-username"])
	registryPassword := string(registrySecret.Data["registry-password"])
	if registry == "" || registryUsername == "" || registryPassword == "" {
		log.Info("Registry credentials not set!")
		return ctrl.Result{}, fmt.Errorf("registry credentials not set")
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
		log.Info("Creating a Secret for deployment", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
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

	updateDeployment := false
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(registryUsername, image) {
			log.Info("Starting image backup", "image", fmt.Sprintf("%s/%s/%s", registry, registryUsername, image))
			newImage, err := ImageBackup(registry, registryUsername, image)
			if err != nil {
				log.Error(err, "Unable to backup image")
				return ctrl.Result{}, err
			}
			log.Info("Replacing image", fmt.Sprintf("%s/%s/%s", registry, registryUsername, image), newImage)
			deployment.Spec.Template.Spec.Containers[i].Image = newImage
			updateDeployment = true
		}
	}

	if updateDeployment {
		log.Info("Updating deployment")
		err = r.Update(ctx, deployment)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
