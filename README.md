# radix-operator

For more background of process, see:
https://github.com/equinor/radix-private/blob/master/docs/how-we-work/development-practices.md

## Release to Cluster

### Radix-pipeline:

We need to build from both master (used by QA environment) and release (used by Prod environment) in both dev and prod subscription. We should not merge to release branch before QA has passed:
For each subscription:
1. git checkout \<branch\>
2. make deploy-pipeline ENVIRONMENT=prod|dev

### Radix-operator:
For development/staging we need to deploy from master branch while for production we need to deploy from release branch
1. Go to cluster inside correct subscription
2. git checkout \<branch\>
2. make helm-up ENVIRONMENT=prod|dev (this will build, push to ACR and release to cluster)

### Operator helm chart


## Updating RadixApplication CRD

The `client-go` SDK requires strongly typed objects when dealing with CRDs so when you add a new type to the spec, you need to update `pkg/apis/radix/v1/types.go` typically.
In order for these objects to work with the SDK, they need to implement certain functions and this is where you run the `code-generator` tool from Kubernetes.
Make sure you `dep ensure` before doing this (you probably did this already to build the operator) as that will pull down the `code-generator` package.
This will auto-generate some code and implement certain interfaces.

        make code-gen

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/client` directory.

This file/directory should NOT be edited.

If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)
