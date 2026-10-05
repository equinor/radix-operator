# kubemerge

Merges a typed `corev1.PodTemplateSpec` overlay onto a base template, e.g. operator config + Radix-generated values.

## How

Kubernetes [strategic merge patch](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/) using the `patchMergeKey`/`patchStrategy` tags on the k8s types:

- Lists with a merge key are merged (`containers`, `env`, `volumes`, `imagePullSecrets` by name, `ports` by `containerPort`, `volumeMounts` by `mountPath`).
- Maps are merged by key; scalars set in overlay win.
- Atomic lists (`command`, `args`, `tolerations`) are replaced.

## Hardcoded nil defaults

Some fields lack `omitempty`, so an unset overlay field marshals to `null`, which strategic merge treats as *delete*. Known cases are defaulted before merging so unset never clears base:

- `spec.containers` → `[]` (merged by name, so empty is a no-op).
- `spec.volumes[].projected.sources` → base sources (atomic list, so `[]` would replace).

Probe handlers (`exec`, `httpGet`, `tcpSocket`, `grpc`) are merged field-by-field, so an overlay handler is added next to the base handler. A nil field is omitted from the patch and cannot clear it; overlays must use the same handler type as base.

Other fields without `omitempty` are not handled; add them here if needed.

`encoding/json` (v1) is used deliberately; json/v2 drops empty structs like `emptyDir: {}`.
