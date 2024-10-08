![Build Status](https://github.com/equinor/radix-operator/workflows/radix-operator-build/badge.svg)  [![SCM Compliance](https://scm-compliance-api.radix.equinor.com/repos/equinor/radix-operator/badge)](https://developer.equinor.com/governance/scm-policy/)
  
# radix-operator

The radix-operator is the central piece of the [Radix platform](https://github.com/equinor/radix-platform) which fully manages the Radix platform natively on [Kubernetes](https://kubernetes.io/). It manages seven [custom resource definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/):

- RR - Application registrations
- RA - Application definition/configuration
- RD - Application deployment
- RJ - Application build/deploy jobs
- RE - Application environments
- RAL - Alerts
- RB - Batch jobs

The `radix-operator` and `radix-pipeline` are built using Github actions, then the `radix-operator` is deployed to cluster through a Helm release using the [Flux Operator](https://github.com/weaveworks/flux) whenever a new image is pushed to the container registry for the corresponding branch. Build and push to container registry is done using Github actions.

## Developer information

### Development Process

The operator is developed using trunk-based development. The two main components here are `radix-operator` and `radix-pipeline`. There is a different setup for each cluster:

- `master` branch should be used for deployment to `dev` clusters. When a pull request is approved and merged to `master`, Github actions build will create a `radix-operator:master-latest` image and push it to ACR. Flux then installs it into the cluster.
- `release` branch should be used for deployment to `playground` and `prod` clusters. When a pull request is approved and merged to `master`, and tested ok in `dev` cluster, we should immediately merge `master` into `release` and deploy those changes to `playground` and `prod` clusters, unless there are breaking changes which needs to be coordinated with release of our other components. When the `master` branch is merged to the `release` branch, Github actions build will create a `radix-operator:release-latest` image and push it to ACR. Flux then installs it into the clusters.

The `radix-pipeline` never gets deployed to cluster, but rather is invoked by the `radix-api`, and the environment mentioned below is the Radix environment of `radix-api` (different environments for `radix-api` therefore use different images of `radix-pipeline`). Both environments are relevant for both `dev`/`playground` as well as `prod`. The process for deploying `radix-pipeline` is this:

- `master` branch should be used for creating the image used in the `qa` environment of any cluster. When a pull request is approved and merged to `master`, Github actions build will create a will create a `radix-pipeline:master-latest` image available in ACR of the subscription
- `release` branch should be used for image used in the `prod` environment of any cluster. When a pull request is approved and merged to `master`, and tested ok in `qa` environment of any cluster, we should immediately merge `master` into `release` and build image used in the `prod` environment of any cluster, unless there are breaking changes which needs to be coordinated with release of our other components. When the `master` branch is merged to the `release` branch, Github actions build will create a `radix-pipeline:release-latest` image available in ACR of the subscription.

Want to contribute? Read our [contributing guidelines](./CONTRIBUTING.md)

### Dependencies management

As of 2019-10-28, radix-operator uses go modules. See [Using go modules](https://blog.golang.org/using-go-modules) for more information and guidelines.

### Generating mocks
We use gomock to generate mocks used in unit test.
You need to regenerate mocks if you make changes to any of the interface types used by the application.
```
make mocks
```

### Procedure to release to cluster

The radix-operator and code is referred to from radix-api through go modules. We follow the [semantic version](https://semver.org/) as recommended by [go](https://blog.golang.org/publishing-go-modules).
radix-operator has three places to set version:
* `version` in `charts/radix-operator/Chart.yaml` - indicate changes in `Chart`
* `appVersion` in `charts/radix-operator/Chart.yaml` - indicates changes in radix-operator logic
* `tag` in git repository - matching to the version of `appVersion` in `charts/radix-operator/Chart.yaml`

`version` and `appVersion` is shown for radix-operator when running command `helm list`

`appVersion` is shown in swagger-ui of the radix-operator API.

To publish a new version of radix-operator:
- When Pool Request with changes is reviewed and code is ready to merge to `mastert` branch - change `version` and/or `appVersion` in `charts/radix-operator/Chart.yaml`
- After merging to the `master` branch - switch to `master` branch and run commands:
```
go mod tidy
make test
git tag v1.0.0
git push origin v1.0.0
```

It is then possible to reference radix-operator from radix-api through adding `github.com/equinor/radix-operator v1.0.0` to the go.mod file.

#### Radix-pipeline

We need to build from both `master` (used by QA environment) and `release` (used by Prod environment) in both `dev` and `prod` subscriptions. We should not merge to `release` branch before QA has passed. Merging to `master` or `release` branch will trigger Github actions build that handles this procedure. The radix-pipeline make use of:

- [radix-tekton](https://github.com/equinor/radix-tekton)
- [radix-image-builder](https://github.com/equinor/radix-image-builder)

#### Radix-operator

For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed. Merging to `master` or `release` branch will trigger Github actions build that handles this procedure.

#### Operator helm chart

For changes to the chart the same procedure applies as for changes to the code. For development/staging we need to deploy from `master` branch while for production we need to deploy from `release` branch. We should not merge to `release` branch before QA has passed. We should never release Helm chart to `playground` or `prod` cluster, as this is now completely handled by Flux operator. If we want to test chart changes we need to disable Flux operator in the development cluster and use the following procedure to release the chart into the cluster:

1. Go to development cluster
2. `git checkout <branch>`
3. Onetime
    ```
        helm init --client-only
    ```
4. for `master` branch: `make helm-up ENVIRONMENT=dev` (will release latest version of helm chart in ACR to cluster)
5. for `custom` branch: 
    ```
        make build-operator ENVIRONMENT=dev
        make helm-up ENVIRONMENT=dev OVERIDE_BRANCH=true
    ``` 

### Updating Radix CRD types

The `client-go` SDK requires strongly typed objects when dealing with CRDs so when you add a new type to the spec, you need to update `pkg/apis/radix/v1/types.go` typically.
In order for these objects to work with the SDK, they need to implement certain functions and this is where you run the `code-generator` tool from Kubernetes.
Generate the strongly type client for Radix v1 objects. `code-gen` will automatically install the required tooling:
```shell
make code-gen
```

This will generate `pkg/apis/radix/v1/zz_generated.deepcopy.go` and `pkg/client` directory.

This file/directory should NOT be edited.

CRD yaml files are generated with [controller-gen(https://pkg.go.dev/sigs.k8s.io/controller-tools/cmd/controller-gen), and are stored in the `charts/radix-operator/templates` directory.
The CRD schema generates use comment markers. Read more about supported markers [here](https://book.kubebuilder.io/reference/markers/crd-validation.html).
Generate CRD yaml files whenever you make changes to any of the types in `pkg/apis/radix/v1/`.

Currently, only the CRD for RadixBatch and RadixApplication is generated.
```shell
make crds
```
This will also regenerate a json schema for RadixApplication into ./json-schema/radixapplication.json.
This schema can be used in code editors like VS Code to get auto complete and validation when editing a radixconfig.yaml file.


If you wish more in-depth information, [read this](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)

## Security Principle

The radix-operator reacts on events to the custom resource types defined by the platform, the RadixRegistration, the RadixApplication, the RadixDeployment, the RadixJob and the RadixEnvironment. It cannot be controlled directly by any platform user. It's main purpose is to create the core resources when the custom resources appears, which will live inside application and environment [namespaces](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/) for the application. Access to a namespace is configured as [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) manifests when the namespace is created, which main purpose is to isolate the platform user applications from one another. For more information on this see [this](./docs/RBAC.md). Another is to define the [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/), to ensure no [pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/) can access another pod, outside of its namespace.

## Automated build and deployment

### Pull request checking

The radix-operator makes use of [GitHub Actions](https://github.com/features/actions) for build checking in every pull request to the `master` branch. Refer to the [configuration file](https://github.com/equinor/radix-operator/blob/master/.github/workflows/radix-operator-pr.yml) of the workflow for more details.

### Build and deploy

The radix-operator utilizes [Github actions build](https://github.com/features/actions) for automated build and push to container registries (ACR) when a branch is merged to `master` or `release` branch. Refer to the [configuration file](https://github.com/equinor/radix-operator/blob/master/.github/workflows/build-push.yml) for more details
