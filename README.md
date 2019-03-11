# radix-operator

## Process

The operator is developed using trunk-based development. The two applications here are `radix-operator` and `radix-pipeline`. They are deployed by downloading and running the correct pre-built images from the container registry. 

For the `radix-pipeline` we only produce a new image when changes are made to the code. `radix-pipeline` is only invoked by `radix-api` application, and the "environment" mentioned below is the Radix environment of `radix-api` (different environments for `radix-api` therefore use different images of `radix-pipeline`. The process for deploying `radix-pipeline` is this:

- `master` branch should be used for creating the image used in the `qa` environment of any cluster. When a pull request is approved and merged to `master`, we should immediately release a new image to be used by the `qa` environment, by (1) checkout and pull `master` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:master-latest` image available in ACR of the subscription
- `release` branch should be used for image used in the `prod` environment of any cluster. When a pull request is approved and merged to `master`, and tested ok in `qa` environment of any cluster we should immediately merge `master` into `release` and build image used in the `prod` environment of any cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) checkout and pull `release` branch (2) `make deploy-pipeline ENVIRONMENT=prod|dev` which will create a `radix-pipeline:release-latest` image available in ACR of the subscription

For the `radix-operator`, instead of releasing to different environments, we release to different clusters:

- `master` branch should be used for deployment to the `dev` cluster. When a pull request is approved and merged to `master`, we should immediately release those changes to the `dev` cluster, by (1) position yourself in the `dev` cluster (2) checkout and pull `master` branch (3) `make helm-up ENVIRONMENT=prod|dev` which will create a `radix-operator:master-latest` image and install it into the `dev` cluster
- `release` branch should be used for deployment to the `prod` cluster. When a pull request is approved and merged to `master`, and tested ok in `dev` cluster we should immediately merge `master` into `release` and deploy those changes to the `prod` cluster, unless these are breaking changes which needs to be coordinated with release of our other components. Release by (1) position yourself in the `prod` cluster (2) checkout and pull `release` branch (3) `make helm-up ENVIRONMENT=prod|dev` which will create a `radix-operator:release-latest` image and install it into the cluster

**Important**: The `radix-api` repo has a dependency to the `radix-operator` repo. As they are currently separated from one another, any change to the `radix-operator` repo (especially related to *spec* change) should be copied to the `vendor` directory of the `radix-api` repo.

## Procedure to test in cluster

It is suggested that testing is conducted in `Omnia Radix Development` subscription during development.

### Radix-pipeline

Testing `radix-pipeline` is carried out by building the pipeline image and pushing it to the container registry. This process can be executed from a local branch while specifying `BRANCH=master` as follows.

```
make deploy-pipeline ENVIRONMENT=dev BRANCH=master
```

### Radix-operator

Testing `radix-operator` can be conducted from a local branch by bypassing the branch-environment constraint using the `OVERIDE_BRANCH=true` flag as follows.

```
make helm-up ENVIRONMENT=dev OVERIDE_BRANCH=true
```

## Procedure to release to cluster

### Radix-pipeline

We need to build from both `master` (used by QA environment) and `release` (used by Prod environment) in both `dev` and `prod` subscriptions. We should not merge to `release` branch before QA has passed.
For each subscription:

```
1. git checkout <branch>
2. make deploy-pipeline ENVIRONMENT=prod|dev
```

### Radix-operator

For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed.

```
1. Go to cluster inside correct subscription
2. git checkout <branch>
3. make helm-up ENVIRONMENT=prod|dev (this will build, push to ACR and release to cluster)
```

### Operator helm chart

For changes to the chart the same procedure applies as for changes to the code. For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed.

```
1. Go to cluster inside correct subscription
2. git checkout <branch>
3. make helm-upgrade-operator-chart ENVIRONMENT=prod|dev (will package and push to ACR)
4. make deploy-via-helm ENVIRONMENT=prod|dev (will release latest version of helm chart in ACR to cluster)
```

## Updating RadixApplication CRD

The `client-go` SDK requires strongly typed objects when dealing with CRDs so when you add a new type to the spec, you need to update `pkg/apis/radix/v1/types.go` typically.
In order for these objects to work with the SDK, they need to implement certain functions and this is where you run the `code-generator` tool from Kubernetes.
Make sure you `dep ensure` before doing this (you probably did this already to build the operator) as that will pull down the `code-generator` package.
This will auto-generate some code and implement certain interfaces.

        make code-gen

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/client` directory.

This file/directory should NOT be edited.

If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)
