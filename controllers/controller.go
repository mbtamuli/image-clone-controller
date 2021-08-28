package controllers

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/go-logr/logr"
	credentialprovider "github.com/vdemeester/k8s-pkg-credentialprovider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func skipNamespace(namespace string) bool {
	return strings.Contains(os.Getenv("EXCLUDED_NAMESPACES"), namespace)
}

func logInRegistry() error {
	registry := os.Getenv("REGISTRY")
	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	return RegistryLogin(registry, registryUsername, registryPassword)
}

func ensureSecret(ctx context.Context, client client.Client, req ctrl.Request, log logr.Logger, registry, registryUsername, registryPassword string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := client.Get(ctx, req.NamespacedName, secret)
	if err != nil && errors.IsNotFound(err) {
		secret, err := newDockerCfgSecret(req.Namespace, req.Name, registry, registryUsername, registryPassword)
		if err != nil {
			return nil, err
		}
		log.Info("Creating a Secret for daemonset imagePullSecret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		if err := client.Create(ctx, secret); err != nil {
			return nil, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		return nil, err
	}
	return secret, nil
}

// newSecret returns a Secret of type kubernetes.io/dockerconfigjson
func newDockerCfgSecret(namespace, name, registry, registryUsername, registryPassword string) (*corev1.Secret, error) {
	dockerConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{},
	}

	dockercfg := credentialprovider.DockerConfigJSON{
		Auths: map[string]credentialprovider.DockerConfigEntry{
			registry: {
				Username: registryUsername,
				Password: registryPassword,
			},
		},
	}

	dockercfgContent, err := json.Marshal(&dockercfg)
	if err != nil {
		return nil, err
	}
	dockerConfigSecret.Data[corev1.DockerConfigJsonKey] = dockercfgContent

	return dockerConfigSecret, nil
}
