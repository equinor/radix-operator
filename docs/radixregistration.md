# RadixRegistration

The purpose of this CRD is to register an application with Radix and bootstrap the process.

## Sample definition

```yaml
apiVersion: radix.equinor.com/v1
kind: RadixRegistration
metadata:
  name: myapp
spec:
  cloneURL: "git@github.com:equinor/myapp"
  sharedSecret: "ThisIsASecret"
  deployKey: ""
  adGroups: 
  - 1asddf33-1asdfa32-2asdfa43-12asdf3
  - werwwer3-asasfdfa-12a2f3-32asdf231
```
