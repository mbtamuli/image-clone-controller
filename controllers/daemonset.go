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

// DaemonsetReconciler reconciles a DaemonSet object
type DaemonsetReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	ExcludeNamespaces string
}

//+kubebuilder:rbac:groups=apps,resources=deployment,verbs=list;watch;update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=list;watch;update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;get;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DaemonsetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("daemonset", req.NamespacedName)
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

	// Fetch the DaemonSet instance
	daemonset := &appsv1.DaemonSet{}
	err = r.Client.Get(ctx, req.NamespacedName, daemonset)
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
	secret, err := ensureSecret(ctx, r.Client, req, log, registry, registryUsername, registryPassword)
	if err != nil {
		return ctrl.Result{}, err
	} else {
		return ctrl.Result{Requeue: true}, nil
	}
	daemonset.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: secret.Name}}

	containers := daemonset.Spec.Template.Spec.Containers
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
			daemonset.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	err = r.Update(ctx, daemonset)
	if err != nil {
		log.Error(err, "Failed to update DaemonSet", "DaemonSet.Namespace", daemonset.Namespace, "DaemonSet.Name", daemonset.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DaemonsetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DaemonSet{}).
		Complete(r)
}
