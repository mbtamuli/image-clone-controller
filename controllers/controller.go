package controllers

import (
	"encoding/json"
	"os"
	"strings"

	credentialprovider "github.com/vdemeester/k8s-pkg-credentialprovider"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func skipNamespace(namespace string) bool {
	return strings.Contains(os.Getenv("EXCLUDED_NAMESPACES"), namespace)
}

func logInRegistry() error {
	if os.Getenv("SKIP_LOGIN") == "true" {
		return nil
	}
	registry := os.Getenv("REGISTRY")
	registryUsername := os.Getenv("REGISTRY_USERNAME")
	registryPassword := os.Getenv("REGISTRY_PASSWORD")

	return RegistryLogin(registry, registryUsername, registryPassword)
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

func deploymentReady(deployments *appsv1.Deployment) bool {
	status := deployments.Status
	desired := status.Replicas
	ready := status.ReadyReplicas
	if desired == ready && desired > 0 {
		return true
	}
	return false
}

func daemonsetReady(daemonsets *appsv1.DaemonSet) bool {
	status := daemonsets.Status
	desired := status.DesiredNumberScheduled
	ready := status.NumberReady
	if desired == ready && desired > 0 {
		return true
	}
	return false
}
