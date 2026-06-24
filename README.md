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

## Repository layout

- `operator` - The Radix Operator watches changes of radix CRDs and reconciles related native kubernetes resources, like converting a `RadixDeployment` to a kubernetes `Deployment`
- `pipeline-runner` - The Pipeline Runners responsibility is to run pipeline jobs, like build-deploys or promote. Usually produces a new `RadixDeployment`. Is created whenever the Operator and the `RadixJob` resource.
- `api-server` - The Radix API Server is the main entrypoint for our users via either `radix-cli` or `radix-web-console`, usually changes Radix CRDs (like creating a `RadixJob` resource, or changing desired replicas on a `RadixDeployment` resource).
- `job-scheduler` - A Job Scheduler component that is run in user namespaces whenever  a user wants to run Job Components. It creates a new API in their namespace they can query to read status, or create the actuall jobs.
- `webhook` - The Webhook configures kubernetes to validate Radix CRDs whenever they are created or updated, before they are persistent and allowed to trigger changes.

- `e2e` - Itnegration tests that starts a real kubernets cluster with Kind, installs Radix, and runs a few smoketests that are not covered by Unit tests.
- `hack` - misc. build scripts
- `json-schema` - Allows end users to validate their `radixconfig.yaml` schema.
- `pkg` - Shared libraries that other components in this repo depend on. Includes generated static Client APIs.

## Development Process

The `radix-operator` project follows a **trunk-based development** approach.

- Run `make test` and `make test-e2e` to test the operator

### 🔁 Workflow

- **External contributors** should:
  - Fork the repository
  - Create a feature branch in their fork

- **Maintainers** may create feature branches directly in the main repository.

### Generating mocks

We use gomock to generate mocks used in unit test.
You need to regenerate mocks if you make changes to any of the interface types used by the application:

```
make mocks
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

### ✅ Merging Changes

All changes must be merged into the `master` branch using **pull requests** with **squash commits**.

The squash commit message must follow the [Conventional Commits](https://www.conventionalcommits.org/en/about/) specification.

## Release Process

Merging a pull request into `master` triggers the **Prepare release pull request** workflow.  
This workflow analyzes the commit messages to determine whether the version number should be bumped — and if so, whether it's a major, minor, or patch change.  

It then creates two pull requests:

- one for the new stable version (e.g. `1.2.3`), and  
- one for a pre-release version where `-rc.[number]` is appended (e.g. `1.2.3-rc.1`).

---

Merging either of these pull requests triggers the **Create releases and tags** workflow.  
This workflow reads the version stored in `version.txt`, creates a GitHub release, and tags it accordingly.

The new tag triggers the **Build and deploy Docker and Helm** workflow, which:

- builds and pushes a new container image and Helm chart to `ghcr.io`, and  
- uploads the Helm chart as an artifact to the corresponding GitHub release.

## Contribution

Want to contribute? Read our [contributing guidelines](./CONTRIBUTING.md)

## Security

This is how we handle [security issues](./SECURITY.md)
