# Radix Velero Plugin

This plugin is intended to assist Velero in backups/restores, so that we are able to recover the state as it was in the original cluster. Information from this plugin ends up as annotations on the restored object. Currently the supported annotations are:

- equinor.com/velero-restored-status

This annotation will be picked up by the radix-operator, after the restore is done

## Plugin deployment

To deploy your plugin image to a Velero server, there are two options.

### Manual deployment

1. Make sure your image is pushed to a registry that is accessible to your cluster's nodes.
2. Run `velero plugin add <image>`, e.g. `velero plugin add radixdev.azurecr.io/radix-velero-plugin:master-latest`

### Configure in Helm chart

Configure the plugin as an [initContainer](https://artifacthub.io/packages/helm/vmware-tanzu/velero?modal=values&path=initContainers) in the Velero Helm chart.

The version set in `image` should match the installed version of radix-operator.

```yaml
initContainers:
  - name: radix-velero-plugin
    image: ghcr.io/equinor/radix/velero-plugin:x.y.z # Replace x.y.z with the actual version you want to install.
    imagePullPolicy: IfNotPresent
    securityContext:
      readOnlyRootFilesystem: true
      runAsNonRoot: true
    volumeMounts:
      - name: plugins
        mountPath: /target
```