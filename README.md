# radix-operator

The radix-operator is the central piece of the [Radix platform](https://github.com/equinor/radix-platform) which fully manages the Radix platform natively on [Kubernetes](https://kubernetes.io/). It manages three [custom resource definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/):

- RR - Application registrations
- RA - Application definition/configuration
- RD - Application deployment

The `radix-operator` and `radix-pipeline` are built using [Azure Devops](https://dev.azure.com/omnia-radix/radix-operator/_build?definitionId=3), then the `radix-operator` is deployed to cluster through a Helm release using the [Flux Operator](https://github.com/weaveworks/flux) whenever a new image is pushed to the container registry for the corresponding branch. There is also a [build-only](https://dev.azure.com/omnia-radix/radix-operator/_build?definitionId=4) pipeline used for checking pull requests.

[![Build Status](https://dev.azure.com/omnia-radix/radix-operator/_apis/build/status/equinor.radix-operator?branchName=master)](https://dev.azure.com/omnia-radix/radix-operator/_build/latest?definitionId=3&branchName=master)

## Developer information

### Development Process

The operator is developed using trunk-based development. The two main components here are `radix-operator` and `radix-pipeline`. There is a different setup for each cluster:

- `master` branch should be used for deployment to the `dev` and `playground` clusters. When a pull request is approved and merged to `master`, we should immediately release those changes to the clusters, by (1) checkout and pull `master` branch (2) `make deploy-operator ENVIRONMENT=dev` which will create a `radix-operator:master-latest` image and install it into the clusters using Flux
- `release` branch should be used for deployment to the `prod` cluster. When a pull request is approved and merged to `master`, and tested ok in `dev` cluster we should immediately merge `master` into `release` and deploy those changes to the `prod` cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) checkout and pull `release` branch (2) `make deploy-operator ENVIRONMENT=prod` which will create a `radix-operator:release-latest` image and install it into the cluster using Flux

The `radix-pipeline` never gets deployed to cluster, but rather is invoked by the `radix-api`, and the environment mentioned below is the Radix environment of `radix-api` (different environments for `radix-api` therefore use different images of `radix-pipeline`). Both environments are relevant for both `dev`/`playground` as well as `prod`. The process for deploying `radix-pipeline` is this:

- `master` branch should be used for creating the image used in the `qa` environment of any cluster. When a pull request is approved and merged to `master`, we should immediately release a new image to be used by the `qa` environment, by (1) checkout and pull `master` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:master-latest` image available in container registry of the subscription
- `release` branch should be used for image used in the `prod` environment of any cluster. When a pull request is approved and merged to `master`, and tested ok in `qa` environment of any cluster we should immediately merge `master` into `release` and build image used in the `prod` environment of any cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) checkout and pull `release` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:release-latest` image available in ACR of the subscription.

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

## Security Principle

The radix-operator reacts on events to the custom resource types defined by the platform, the RadixRegistration, the RadixApplication and the RadixDeployment. It cannot be controlled directly by any platform user. It's main purpose is to create the core resources when the custom resources appears, which will live inside application and environment [namespaces](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/) for the application. Access to a namespace is configured as [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) manifests when the namespace is created, which main purpose is to isolate the platform user applications from one another. For more information on this see [this](./docs/RBAC.md). Another is to define the [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/), to ensure no [pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/) can access another pod, outside of its namespace.

