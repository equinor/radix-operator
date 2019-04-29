# radix-operator

## Developer information

### Development Process

The operator is developed using trunk-based development. The two main components here are `radix-operator` and `radix-pipeline`. The `radix-operator` is deployed to cluster through a Helm release using the [Flux Operator](https://github.com/weaveworks/flux) whenever a new image is pushed to the container registry. For more information on the setup see [here](https://github.com/equinor/radix-flux). There is a different setup for each cluster:

- `master` branch should be used for deployment to the `dev` and `playground` cluster. When a pull request is approved and merged to `master`, we should immediately release those changes to the clusters, by (1) checkout and pull `master` branch (2) `make deploy-operator ENVIRONMENT=dev` which will create a `radix-operator:master-latest` image and install it into the clusters using Flux
- `release` branch should be used for deployment to the `prod` cluster. When a pull request is approved and merged to `master`, and tested ok in `dev` cluster we should immediately merge `master` into `release` and deploy those changes to the `prod` cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) checkout and pull `release` branch (2) `make deploy-operator ENVIRONMENT=prod` which will create a `radix-operator:release-latest` image and install it into the cluster using Flux

The `radix-pipeline` never gets deployed to cluster, but rather is invoked by the `radix-api`, and the environment mentioned below is the Radix environment of `radix-api` (different environments for `radix-api` therefore use different images of `radix-pipeline`). Both environments are relevant for both `dev`/`playground` as well as `prod`. The process for deploying `radix-pipeline` is this:

- `master` branch should be used for creating the image used in the `qa` environment of any cluster. When a pull request is approved and merged to `master`, we should immediately release a new image to be used by the `qa` environment, by (1) checkout and pull `master` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:master-latest` image available in container registry of the subscription
- `release` branch should be used for image used in the `prod` environment of any cluster. When a pull request is approved and merged to `master`, and tested ok in `qa` environment of any cluster we should immediately merge `master` into `release` and build image used in the `prod` environment of any cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) checkout and pull `release` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:release-latest` image available in ACR of the subscription

**Important**: The `radix-api` repo has a dependency to the `radix-operator` repo. As they are currently separated from one another, any change to the `radix-operator` repo (especially related to *spec* change) should be copied to the `vendor` directory of the `radix-api` repo.

### Procedure to release to cluster

#### Radix-pipeline

We need to build from both `master` (used by QA environment) and `release` (used by Prod environment) in both `dev` and `prod` subscriptions. We should not merge to `release` branch before QA has passed.
For each subscription:

```
1. git checkout <branch>
2. make deploy-pipeline ENVIRONMENT=prod|dev
```

#### Radix-operator

For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed.

```
1. Go to cluster inside correct subscription
2. git checkout <branch>
3. make deploy-operator ENVIRONMENT=prod|dev
```

#### Operator helm chart

For changes to the chart the same procedure applies as for changes to the code. For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed. We should never release Helm chart to `playground` or `prod` cluster, as this is now completely handled by Flux operator. If we want to test chart changes we need to disable Flux operator in the development cluster and use the following proceedure to release the chart into the cluster:

```
1. Go to development cluster
2. git checkout <branch>
3. make helm-up ENVIRONMENT=dev (will release latest version of helm chart in ACR to cluster)
```

### Updating RadixApplication CRD

The `client-go` SDK requires strongly typed objects when dealing with CRDs so when you add a new type to the spec, you need to update `pkg/apis/radix/v1/types.go` typically.
In order for these objects to work with the SDK, they need to implement certain functions and this is where you run the `code-generator` tool from Kubernetes.
Make sure you `dep ensure` before doing this (you probably did this already to build the operator) as that will pull down the `code-generator` package.
This will auto-generate some code and implement certain interfaces.

```
make code-gen
```

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/client` directory.

This file/directory should NOT be edited.

If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)
