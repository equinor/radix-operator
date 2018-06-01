# radix-operator

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

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/apis/client` directory.

This file/directory should NOT be edited.

If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)
