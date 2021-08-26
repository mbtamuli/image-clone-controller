package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/tools/cache"
)

// DockerConfigJSON, DockerConfig and DockerConfigEntry copied from
// https://github.com/kubernetes/kubectl/blob/723266d1458429c5741ec6c0b5c315e72ec6f7cb/pkg/cmd/create/create_secret_docker.go#L64-L83

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

// deploymentSyncHandler compares the actual state with the desired for deployments, and attempts to
// converge the two.
func (c *Controller) deploymentSyncHandler(key string) error {
	c.logger.Info("Starting deployment handler for %s\n", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Error("invalid resource key: %s\n", key)
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.logger.Error("deployment '%s' in work queue no longer exists\n", key)
			return nil
		}
		return err
	}

	c.logger.Info("Preparing Secret object")
	secretApplyConfig := applycorev1.Secret(name, namespace)
	dockerConfigJSONContent, err := handleDockerCfgJSONContent(c.registryPassword, c.registryPassword, c.registry)
	if err != nil {
		return err
	}
	if secretApplyConfig.Data == nil {
		secretApplyConfig.Data = make(map[string][]byte)
	}
	secretApplyConfig.Data[corev1.DockerConfigJsonKey] = dockerConfigJSONContent
	c.logger.Info("Creating Secret")
	dockerSecret, err := c.kubeclientset.CoreV1().Secrets(namespace).Apply(context.TODO(), secretApplyConfig, metav1.ApplyOptions{FieldManager: "image-clone-controller"})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: dockerSecret.Name}}

	containers := deployment.Spec.Template.Spec.Containers
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(c.repository, image) {
			c.logger.Info("Starting image backup for image: %s/%s/%s", c.registry, c.repository, image)
			newImage, err := ImageBackup(c.registry, c.repository, image)
			if err != nil {
				return fmt.Errorf("unable to backup image: %s", err)
			}
			c.logger.Info("Replacing image '%s/%s/%s' with '%s'", c.registry, c.repository, image, newImage)
			deployment.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	c.logger.Info("Updating deployment")
	_, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// daemonsetSyncHandler compares the actual state with the desired for daemonsets, and attempts to
// converge the two.
func (c *Controller) daemonsetSyncHandler(key string) error {
	c.logger.Info("Starting daemonset handler for %s\n", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Error("invalid resource key: %s\n", key)
		return nil
	}

	daemonset, err := c.daemonsetsLister.DaemonSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.logger.Error("daemonset '%s' in work queue no longer exists\n", key)
			return nil
		}
		return err
	}

	c.logger.Info("Preparing Secret object")
	secretApplyConfig := applycorev1.Secret(name, namespace)
	dockerConfigJSONContent, err := handleDockerCfgJSONContent(c.registryPassword, c.registryPassword, c.registry)
	if err != nil {
		return err
	}
	if secretApplyConfig.Data == nil {
		secretApplyConfig.Data = make(map[string][]byte)
	}
	secretApplyConfig.Data[corev1.DockerConfigJsonKey] = dockerConfigJSONContent
	c.logger.Info("Creating Secret")
	dockerSecret, err := c.kubeclientset.CoreV1().Secrets(namespace).Apply(context.TODO(), secretApplyConfig, metav1.ApplyOptions{FieldManager: "image-clone-controller"})
	if err != nil {
		return err
	}

	daemonset.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: dockerSecret.Name}}

	containers := daemonset.Spec.Template.Spec.Containers
	for i, container := range containers {
		image := container.Image
		if !ImageBackedUp(c.repository, image) {
			c.logger.Info("Starting image backup for image: %s/%s/%s", c.registry, c.repository, image)
			newImage, err := ImageBackup(c.registry, c.repository, image)
			if err != nil {
				return fmt.Errorf("unable to backup image: %s", err)
			}
			c.logger.Info("Replacing image '%s/%s/%s' with '%s'", c.registry, c.repository, image, newImage)
			daemonset.Spec.Template.Spec.Containers[i].Image = newImage
		}
	}

	c.logger.Info("Updating daemonsets")
	_, err = c.kubeclientset.AppsV1().DaemonSets(namespace).Update(context.TODO(), daemonset, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
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
