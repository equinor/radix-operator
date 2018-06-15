# RadixRegistration

The purpose of this CRD is to register an application with Radix and bootstrap the process.

## Sample definition

```yaml
apiVersion: radix.equinor.com/v1
kind: RadixRegistration
  metadata:
    name: complete
spec:
  repository: "https://github.com/Statoil/myapp"
  cloneURL: "git@github.com:Statoil/myapp"
  sharedSecret: "ThisIsASecret"
  defaultScript: "" 
  deployKey: ""
```