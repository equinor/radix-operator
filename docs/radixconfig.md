# RadixConfig

In order for Omnia Radix to configure your application it needs the RadixConfig file. 

This file needs to live in the root of your repository and be named `radixconfig.yaml`.

## Sample configuration

```yaml
apiVersion: radix.equinor.com/v1
kind: RadixApplication
metadata:
  name: myapp
spec:
  environments:
    - name: dev
      authorization:
      - role: RadixReader
        groups:
        - "<Azure AD group ID>"
    - name: prod
      authorization:
      - role: RadixReader
        groups:
        - "<Azure AD group ID>"
  components:
    - name: frontend
      src: frontend
      ports:
       - 80
      public: true
    - name: backend
      src: backend
      ports:
        - 5000
      env:
        DB_HOST: "db"
        DB_PORT: "1234"
```

## Specification

### Environments

This is an array of environments for your application.

#### Name

The name of your environment

#### Authorization

A list of rolemappings - mapping Azure AD groups to a role in Radix

Available Roles:
- RadixReader
- RadixDeploy
- RadixAdmin

### Components

This is where you specify the various components for your application - it needs at least one.

#### Name

Name of the component - this will be used for building the images (appName-componentName).

#### Src

The folder where the Dockerfile can be found.

#### Public

true/false - This will generate a public endpoint for the component if set to true.

#### Ports

An array of ports exposed by your component.

#### Env

An array of environment variables to be set inside the running container.