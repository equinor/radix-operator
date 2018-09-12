# radix-operator

## Development on Windows with Windows Subsystem for Linux (WSL)

Follow this tutorial to get Docker working from inside WSL: https://nickjanetakis.com/blog/setting-up-docker-for-windows-and-wsl-to-work-flawlessly

Also handy if you have problems removing docker.io: https://github.com/docker/for-linux/issues/52

The repo also has to be cloned to the correct path under GOPATH. So for example

    export GOPATH=/home/stian/whereIkeepMycode
    mkdir -p $GOPATH/src/github.com/statoil/
    cd $GOPATH/src/github.com/statoil/
    git clone git@github.com:Statoil/radix-operator.git

PS: The local organization path (statoil) HAS to be lowercase. If it is capitalized `dep ensure` will download a copy of `statoil/radix-operator` and put it in your `vendor/` folder as an external dependency and any code changes won't have any effect. It's not possible to use proper capitalization in the Go imports since Kubernetes code-generator will lowercase stuff in the process and fail.

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

## Deployment  to Kubernetes

1. Make Docker image.

        make docker-build

    If this does not work, delete `Gopkg.lock` and `Gopkg.toml` files and run the following command:

        dep init

    If some errors occur, try deleting `$GOPATH/pkg/dep/sources` directory and all its sub-directories, and re-run `dep init`.

2. Push the created Docker image to container registry.

        docker login [REGISTRY] -u [USERNAME] -p [PASSWORD]
        make docker-push

3. Deploy using the given `helm` chart. First, modify `values.yaml` file under `charts/radix-operator` directory, change the `tag` value to the Docker image tag (i.e. the first 7 digits of the commit hash). Modify other parameters as necessary. Then install the chart using the following command.

        helm install -n [NAME] charts/radix-operator

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


# radix-webhook

## Installing Helm Chart

Installing Radix Webhook using the Helm Chart you need to do the following:

- Clone this repository
- Run: `helm inspect values charts/radix-webhook > radix-webhook.yaml`
- Edit the `radix-webhook.yaml` accordingly
- Install: `helm install -n radix-webhook -f radix-webhook.yaml ./charts/radix-webhook`