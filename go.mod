module github.com/mbtamuli/image-clone-controller

go 1.16

require (
	github.com/docker/cli v20.10.8+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/google/go-containerregistry v0.6.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/vdemeester/k8s-pkg-credentialprovider v1.21.0
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	k8s.io/api v0.22.0-rc.0
	k8s.io/apimachinery v0.22.0-rc.0
	k8s.io/client-go v0.22.0-rc.0
	sigs.k8s.io/controller-runtime v0.9.2
)

replace (
	// these are needed since k8s.io/kubernetes cites v0.0.0 for each of these k8s deps in its go.mod
	k8s.io/api => k8s.io/api v0.22.0-rc.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.0-rc.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.0-rc.0
	k8s.io/apiserver => k8s.io/apiserver v0.22.0-rc.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.0-rc.0
	k8s.io/client-go => k8s.io/client-go v0.22.0-rc.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.0-rc.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.0-rc.0
	k8s.io/code-generator => k8s.io/code-generator v0.22.0-rc.0
	k8s.io/component-base => k8s.io/component-base v0.22.0-rc.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.0-rc.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.0-rc.0
	k8s.io/cri-api => k8s.io/cri-api v0.22.0-rc.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.0-rc.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.0-rc.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.0-rc.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.0-rc.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.0-rc.0
	k8s.io/kubectl => k8s.io/kubectl v0.22.0-rc.0
	k8s.io/kubelet => k8s.io/kubelet v0.22.0-rc.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.0-rc.0
	k8s.io/metrics => k8s.io/metrics v0.22.0-rc.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.0-rc.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.0-rc.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.0-rc.0
)
