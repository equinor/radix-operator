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