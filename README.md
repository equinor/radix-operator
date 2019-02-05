# radix-operator

## Release to Cluster

Radix-pipeline:

We need to build from both master (used by QA environment) and release (used by Prod environment)
For each branch:
1. git checkout <branch>
2. 


1. Make Docker image:

    make build

    If this does not work, delete `Gopkg.lock` and `Gopkg.toml` files and run the following command:

    dep init

    If some errors occur, try deleting `$GOPATH/pkg/dep/sources` directory and all its sub-directories, and re-run `dep init`.

2. Push the created Docker image to container registry:

    make push

3. Deploy (using the helm chart in the background.):

    make deploy-via-helm

Will by default deploy image tag with commit id. Optionally deploy another image:

    TAG=6e2da3995c078f33613cf459942d914f88f40367 make deploy-via-helm

4. Combined command for build push & deploy.

    make helm-up

**First time setup - Add private Helm Repo** 

If you have not used private Helm Repos in ACR before you need to add it before you can push and deploy from it:

    az configure --defaults acr=radixdev
    az acr helm repo add
    helm repo update

## Updating RadixApplication CRD

The `client-go` SDK requires strongly typed objects when dealing with CRDs so when you add a new type to the spec, you need to update `pkg/apis/radix/v1/types.go` typically.
In order for these objects to work with the SDK, they need to implement certain functions and this is where you run the `code-generator` tool from Kubernetes.
Make sure you `dep ensure` before doing this (you probably did this already to build the operator) as that will pull down the `code-generator` package.
This will auto-generate some code and implement certain interfaces.

        make code-gen

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/client` directory.

This file/directory should NOT be edited.

If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)

## Installing Helm Chart

Installing Radix Operator using the Helm Chart you need to do the following:

- Clone this repository
- Run: `helm inspect values ./charts/radix-operator > radix-operator.yaml`
- Edit the `radix-operator.yaml` and fill in the credentials for the Container Registry you wish to use
- Install: `helm install -f radix-operator.values ./charts/radix-operator`

If you wish to use a different image version, update the `image.tag` property in `radix-operator.yaml` you created above.
