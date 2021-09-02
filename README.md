# Image Clone Controller

**Goal:** To be safe against the risk of public container images disappearing from the registry while we use them, breaking our deployments.

**Idea:** Have a controller which watches the applications and "caches" the images by re-uploading to a separate registry's repository and reconfiguring the applications to use these copies.

## Demo

[![asciicast](https://asciinema.org/a/433746.png)](https://asciinema.org/a/433746)

## Deployment

```sh
cp controller-config.example controller-config
cp env.example env
# Edit the files controller_config and env with proper details

make deploy ENV_FILE=env CONFIG_FILE=controller-config
```

You can build and deploy your own image using the following commands
```sh
make docker-build docker-push IMG=ghcr.io/mbtamuli/image-clone-controller:0.0.1
make deploy IMG=ghcr.io/mbtamuli/image-clone-controller:0.0.1 ENV_FILE=env CONFIG_FILE=controller-config
```

The project has make targets to deploy to the kubernets cluster. You will need to set the `KUBECONFIG` variable to point to the right configuration. You will also need to set the context correctly using `kubectl config use-context <CONTEXT>`

```
  deploy                     Deploy the controller and related resources to the cluster.
  undeploy                   Remove controller and related resources from the cluster.
```

Once the controller is deployed, any Deployment or DaemonSet created, excluding those in the `EXCLUDED_NAMESPACES`, will have their container images backed up and the Deployment/DaemonSet will be updated to use the backed up image.

### ENV
For deploying, you will need to set a few environment variables. The variables will be used while creating a Secret, which will be used for authenticating to the remote repository.
- `REGISTRY`: Registry to use. Check the table below for the exact value.
- `REGISTRY_USERNAME`: Username for Registry authentication.
- `REGISTRY_PASSWORD`: Password for Registry authentication.
- `REPOSITORY`: Repository to use. For example - `ghcr.io/mbtamuli`, here `mbtamuli` is the repository.


### CONTROLLER-CONFIG
For controlling certain behaviors of the controller.
- `EXCLUDED_NAMESPACES`: Specify the namespaces that the controller should not modify any objects in.
- `SKIP_LOGIN`: If your user has unauthenticated access to push/pull to a repository, you can choose to set `SKIP_LOGIN` to `true`(defaults to `false`).

**Registries**

| Registry                  | Value for environment variable | Tested             |
|---------------------------|:------------------------------:|--------------------|
| DockerHub                 | docker.io                      | :white_check_mark: |
| Quay.io                   | quay.io                        | :white_check_mark: |
| GitHub Container Registry | ghcr.io                        | :white_check_mark: |

## Implementation

The implementation for the controller has the following approach
 - Watch for creation of `Deployment` or `DaemonSet` object.
 - On receiving events for the object's creation, fetch the container(s) image(s) being used by the object.
 - Log into the remote repository.
 - Rename the image in the format - `<REPOSITORY>/<IMAGE_NAME_WITH_/_REPLACED_BY_-_>/<TAG>`.
 - Push the image to the remote repository.
 - Create a `Secret`, of type `kubernetes.io/dockerconfigjson`, in the namespace of the object.
 - Patch the object to use the remote image and the newly created `Secret` as `imagePullSecrets`.


## Development

You can run the controller locally, outside of the cluster using the following commands:
```sh
cp controller-config.example controller-config
cp env.example env
# Edit the files controller_config and env with proper details
source env
export $(cut -d= -f1 env)
source controller-config
export $(cut -d= -f1 controller-config)

make run
```

## Tests

Run tests for the controller using the following command:
```sh
make test

# For verbose output
make test-verbose
```

To deploy it to Kind cluster, you don't need to push the controller image, you can use the following commands
```sh
make docker-build kind-load IMG=ghcr.io/mbtamuli/image-clone-controller:0.0.1
make deploy IMG=ghcr.io/mbtamuli/image-clone-controller:0.0.1 ENV_FILE=env CONFIG_FILE=controller-config
```
