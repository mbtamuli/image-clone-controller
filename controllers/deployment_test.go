package controllers

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Deployment controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout = time.Second * 60
		// interval = time.Millisecond * 250
		interval = time.Second * 10
	)

	BeforeEach(func() {
		ctx := context.Background()

		Expect(os.Getenv("REGISTRY")).NotTo(BeEmpty())
		os.Setenv("REGISTRY_USERNAME", "")
		os.Setenv("REGISTRY_PASSWORD", "")
		os.Setenv("SKIP_LOGIN", "true")

		Expect(k8sClient.Create(ctx, getDeployment("test", "default"))).Should(Succeed())
	})

	AfterEach(func() {
		ctx := context.Background()
		Expect(k8sClient.Delete(ctx, getDeployment("test", "default"))).Should(Succeed())
	})

	Context("When a deployment is created", func() {
		It("Should backup the image", func() {
			ctx := context.Background()

			Eventually(func() bool {
				deploymentLookupKey := types.NamespacedName{Name: "test", Namespace: "default"}
				updatedDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, deploymentLookupKey, updatedDeployment)).Should(Succeed())

				containers := updatedDeployment.Spec.Template.Spec.Containers
				for _, container := range containers {
					image := container.Image
					if !ImageBackedUp(os.Getenv("REGISTRY"), image) {
						return false
					}
					Logf("Image backed up: %s", image)
				}
				return true
			}, timeout, interval).Should(BeTrue())

		})

		It("Should ignore the namespaces in the EXCLUDED_NAMESPACES", func() {
			ctx := context.Background()

			os.Setenv("EXCLUDED_NAMESPACES", "other-deployment")

			otherNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other-deployment",
				},
			}
			Expect(k8sClient.Create(ctx, otherNamespace)).Should(Succeed())

			Expect(k8sClient.Create(ctx, getDeployment("test", "other-deployment"))).Should(Succeed())

			Eventually(func() bool {
				deploymentLookupKey := types.NamespacedName{Name: "test", Namespace: "other-deployment"}
				updatedDeployment := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, deploymentLookupKey, updatedDeployment)).Should(Succeed())

				containers := updatedDeployment.Spec.Template.Spec.Containers
				for _, container := range containers {
					image := container.Image
					if ImageBackedUp(os.Getenv("REGISTRY"), image) {
						return true
					}
					Logf("Image not backed up for deployment in excluded namespace: %s", "other-deployment")
				}
				return false
			}, timeout, interval).ShouldNot(BeTrue())

			Expect(k8sClient.Delete(ctx, getDeployment("test", "other-deployment"))).Should(Succeed())
			Expect(k8sClient.Delete(ctx, otherNamespace)).Should(Succeed())

		})
	})

})

func getDeployment(name, namespace string) *appsv1.Deployment {
	replicas := int32(1)
	labels := map[string]string{"app": "nginx"}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "quay.io/mbtamuli/nginx:1.19",
						},
						{
							Name:    "busybox",
							Image:   "quay.io/mbtamuli/busybox:1.34.0",
							Command: []string{"/bin/sh"},
							Args:    []string{"-c", "sleep", "3600"},
						},
					},
				},
			},
		},
	}
}
