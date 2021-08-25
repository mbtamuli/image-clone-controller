# Image Clone Controller

**Goal:** To be safe against the risk of public container images disappearing from the registry while we use them, breaking our deployments.

**Idea:** Have a controller which watches the applications and "caches" the images by re-uploading to a separate registry's repository and reconfiguring the applications to use these copies.

## Deployment

The project has make targets to deploy to the kubernets cluster. You will need to set the `KUBECONFIG` variable to point to the right configuration. You will also need to set the context correctly using `kubectl config use-context <CONTEXT>`

```
  deploy                     Deploy the controller and related resources to the cluster.
  undeploy                   Remove controller and related resources from the cluster.
```

For deploying, you will need to set a few environment variables. The variables will be used while creating a Secret, which will be used for authenticating to the remote repository. The controller can be set to not modify any objects in specified namespaces. The default excluded namespaces are - `"kube-system,local-path-storage,image-clone-controller"`.

```sh
//Check the table below for the exact value for REGISTRY
export REGISTRY="CHANGE_ME"
export REGISTRY_USERNAME="CHANGE_ME"
export REGISTRY_PASSWORD="CHANGE_ME"

// OPTIONAL
export EXCLUDE_NAMESPACES="list,of,comma,separated,namespaces"
make deploy
```

**Registries**

| Registry                  | Value for environment variable | Tested             |
|---------------------------|:------------------------------:|--------------------|
| DockerHub                 | docker.io                      | :white_check_mark: |
| Quay.io                   | quay.io                        | :white_check_mark: |
| GitHub Container Registry | ghcr.io                        | :white_check_mark: |

## Implementation

The implementation for the controller has the following approach
 - Watch for creation of Deployments or Daemonsets object.
 - On receiving events for the object's creation, fetch the container(s) image(s) being used by the object.
 - Log into the remote repository.
 - Rename the image in the format - `<REPOSITORY>/<IMAGE_NAME_WITH_/_REPLACED_BY_-_>/<TAG>`.
 - Push the image to the remote repository.
 - Create a Secret, of type `kubernetes.io/dockerconfigjson`, in the namespace of the object.
 - Patch the object to use the remote image and the newly created secret as `imagePullSecrets`.


## Development

For development, **kind** cluster was used, and the local-deploy target works for kind. Other targets help in setting up the local-cluster and deploying to it.

```
  start-local-cluster        Start kind cluster.
  stop-local-cluster         Stop kind cluster.
  local-deploy               Deploy to cluster for development, mounting the current directory inside the cluster.
  kind-load-docker           Load docker-image in kind cluster.
```

You can also deploy the controller outside the cluster. This approach is helpful during development. There are targets to help with builds.
```
  clean                      Clean build artifacts.
  fmt                        Run go fmt.
  vet                        Run go vet.
  build                      Build the controller binary.
  docker-build               Build controller docker image.
  docker-push                Push controller docker image.
```

You can run the controller outside the cluster with the following command
```sh
./image-clone-controller \
    -registry="$REGISTRY" \
    -registry-username="$REGISTRY_USERNAME" \
    -registry-password="$REGISTRY_PASSWORD" \
    -repository "CHANGE_ME" \
    -exclude-namespaces="CHANGE_ME"
```

For example, for development with the kind cluster, I used
```sh
./image-clone-controller \
    -registry="ghcr.io" \
    -registry-username="mbtamuli" \
    -registry-password="$(pass Cloud/ghcr.io/mbtamuli)" \
    -repository "mbtamuli" \
    -exclude-namespaces="kube-system,local-path-storage,image-clone-controller"
```
