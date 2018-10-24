# RadixConfig

In order for Omnia Radix to configure your application it needs the RadixConfig file. 

This file needs to live in the root of your repository and be named `radixconfig.yaml`.
The name of the application needs to match its corresponding [RadixRegistration](radixregistration.md).

## Sample configuration

```yaml
apiVersion: radix.equinor.com/v1
kind: RadixApplication
metadata:
  name: myapp
spec:
  environments:
    - name: dev
      build:
        from: master
    - name: prod
  components:
    - name: frontend
      src: frontend
      ports:
       - name: http
         port: 80
      public: true
    - name: backend
      src: backend
      replicas: 2
      ports:
        - name: http
          port: 5000
      environmentVariables:
        - environment: dev
          variables:
            DB_HOST: "db-dev"
            DB_PORT: "1234"
        - environment: prod
          variables:
            DB_HOST: "db-prod"
            DB_PORT: "9876"
      secrets:
        - DB_PASS
```

## Specification

### environments

This is an array of environments for your application. 

#### name

The name of your environment

#### build

You have to specify the branch name to build and deploy in each environment by adding `from: <BRANCH_NAME>`. If you do not specify one in an environment, nothing will be built and deployed to this particular environment.

In the example above, a `git push` to `master` branch will build and deploy code from the `master` branch to `dev` environment. Nothing will be built and deployed to the `prod` environment.

### components

This is where you specify the various components for your application - it needs at least one.

#### name

Name of the component - this will be used for building the images (appName-componentName).

#### src

The folder where the Dockerfile can be found.

#### replicas

Scales the component

#### public

true/false - This will generate a public endpoint for the component if set to true.

#### ports

An array of ports exposed by your component.

#### environmentVariables

An array of objects containing environment name and variables to be set inside the running container.

#### secrets

An array of keys that should be saved as a Kubernetes `secret` object. The `secret` object should be created in every environment (i.e. `namespace`).

```
kubectl create secret generic backend -n myapp-dev --from-literal=DB_PASS=devpassword

kubectl create secret generic backend -n myapp-prod --from-literal=DB_PASS=prodpassword
```