package controllers

import (
	"context"
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DaemonsetReconciler reconciles a DaemonSet object
type DaemonsetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployment,verbs=list;watch;update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=list;watch;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;get;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DaemonsetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("daemonset", req.NamespacedName)

	if skipNamespace(req.Namespace) {
		return ctrl.Result{}, nil
	}

	if err := logInRegistry(); err != nil {
		log.Error(err, "Failed to log into registry")
		return ctrl.Result{}, nil
	}

	registry := os.Getenv("REGISTRY")
	repository := os.Getenv("REPOSITORY")
	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	// Fetch the DaemonSet instance
	daemonset := &appsv1.DaemonSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, daemonset); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	secret := &corev1.Secret{}
	// Check if the secret already exists, if not create a new one
	if registryUsername != "" && registryPassword != "" {
		err := r.Client.Get(ctx, req.NamespacedName, secret)
		if err != nil && errors.IsNotFound(err) {
			secret, err := newDockerCfgSecret(req.Namespace, req.Name, registry, registryUsername, registryPassword)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Creating a Secret for daemonset imagePullSecret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
			if err := r.Client.Create(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
			// Secret created successfully - return and requeue
			log.Info("Secret created successfully")
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "Failed to get Secret")
			return ctrl.Result{}, nil
		}
	}

	updatedDaemonset := daemonset.DeepCopy()
	// Ensure the daemonset has ImagePullSecrets
	if !reflect.DeepEqual(daemonset.Spec.Template.Spec.ImagePullSecrets, []corev1.LocalObjectReference{{Name: secret.Name}}) {
		log.Info("Updating ImagePullSecret with", "Secret", secret.Name)
		updatedDaemonset.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: secret.Name}}
		if !daemonsetReady(daemonset) {
			log.Info("Waiting for daemonset to be ready")
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		}
		if err := r.Update(ctx, updatedDaemonset); err != nil {
			log.Error(err, "Failed to update DaemonSet", "DaemonSet.Namespace", updatedDaemonset.Namespace, "DaemonSet.Name", updatedDaemonset.Name)
			return ctrl.Result{}, err
		}
		log.Info("Deployment ImagePullSecrets updated successfully")
		return ctrl.Result{Requeue: true}, nil
	}

	containers := updatedDaemonset.Spec.Template.Spec.Containers
	updateDeployment := true
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(registry, image) {
			log.Info("Image Backed Up", "Old image", image)
			newImage, err := ImageBackup(registry, repository, image)
			if err != nil {
				updateDeployment = false
				log.Error(err, "Unable to backup image")
				return ctrl.Result{}, err
			}
			log.Info("Image Backed Up", "New image", newImage)
			updatedDaemonset.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	if updateDeployment {
		if !daemonsetReady(daemonset) {
			log.Info("Waiting for daemonset to be ready")
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		}
		if err := r.Update(ctx, updatedDaemonset); err != nil {
			log.Error(err, "Failed to update DaemonSet", "DaemonSet.Namespace", updatedDaemonset.Namespace, "DaemonSet.Name", updatedDaemonset.Name)
			return ctrl.Result{}, err
		}
		log.Info("All images of daemonset have been backed up.")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DaemonsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DaemonSet{}).
		Complete(r)
}
