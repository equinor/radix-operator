# radix-operator

## Development on Windows with Windows Subsystem for Linux (WSL)

Follow this tutorial to get Docker working from inside WSL: https://nickjanetakis.com/blog/setting-up-docker-for-windows-and-wsl-to-work-flawlessly

Also handy if you have problems removing docker.io: https://github.com/docker/for-linux/issues/52

The repo also has to be cloned to the correct path under GOPATH. So for example

    export GOPATH=/home/stian/whereIkeepMycode
    mkdir -p $GOPATH/src/github.com/equinor/
    cd $GOPATH/src/github.com/equinor/
    git clone git@github.com:equinor/radix-operator.git

PS: The local organization path (equinor) HAS to be lowercase. If it is capitalized `dep ensure` will download a copy of `equinor/radix-operator` and put it in your `vendor/` folder as an external dependency and any code changes won't have any effect. It's not possible to use proper capitalization in the Go imports since Kubernetes code-generator will lowercase stuff in the process and fail.

Create a link so that make can find GoMetaLinter

    ln -s /root/go/bin/gometalinter /usr/bin/gometalinter

Also, in vendor/k8s.io/client-go/plugin/pkg/client/auth/azure/azure.go:300

Change

    token:       spt.Token,

To

    token:       spt.Token(),

This because we cannot use latest version of client-go because reasons.

If the build complains about missing a git tag, add a tag manually with

    git tag v1.0.0

Then do `make docker-build` and after that completes `go run radix-operator/main.go` should also work locally.

## Deployment to Kubernetes

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