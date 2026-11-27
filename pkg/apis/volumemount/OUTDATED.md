# volumemount: how it used to work

This document describes how volume storage was handled before the rewrite of the `volumemount` package. It is kept for historical reference only. See [README.md](./README.md) for how it works now.

## Overview

Storage was handled by `GetVolumes`, `CreateOrUpdatePVCVolumeResourcesForDeployComponent` (in `pvcvolume.go`) and `GarbageCollectCsiAzureVolumeResourcesForDeployComponent`.

1. **Two-pass volume build.** `GetVolumes` built the Pod volumes and copied the volume source of any current workload volume with the same name. Otherwise it generated a new claim name, `pvc-<volumeName>-<random5>`. Then `CreateOrUpdatePVCVolumeResourcesForDeployComponent` went through those volumes and tried to reconcile the storage behind each claim name.
2. **Name-driven reconciliation with random names.** The reconciler:
   - looked up the PV whose `claimRef.name` matched the claim name, and compared it with a freshly built PV (`ComparePersistentVolumes`, which allowed the random suffix of the claim name to differ);
   - if that PV did not exist, scanned all CSI PVs in the namespace for one with matching labels and spec;
   - checked the PVC with `ComparePersistentVolumeClaims`.

   If anything differed, it created a new PVC with a new random suffix and a new PV named `pv-radixvolumemount-<uuid>`, with a UUID in the volumeHandle. A "HACK" forced the PV to be recreated when it had a storage class but its PVC was missing.
3. **Deprecated volume type.** `type: azure-blob` (with the top-level `storage`, `uid`, `gid`, `accessMode`, `requestsStorage` fields) had its own PV spec builder.
4. **Garbage collection per component.** GC ran inside `syncDeploymentForRadixComponent`, once for every component, before the other components were synced. It:
   - listed all Deployments and Jobs in the namespace and collected the claim names in their Pod templates;
   - deleted **every** PVC in the namespace that was not referenced, with no label filter, together with the PV named in `volumeName`;
   - deleted every CSI PV whose `claimRef.namespace` was this namespace and whose claim was not referenced, in any phase;
   - deleted `Released`/`Failed` Blob CSI PVs **cluster-wide** whose `claimRef.name` was not in this namespace's referenced set.

   Deletes had no preconditions. Volumes for RadixBatches were not considered, except through Jobs that already existed. Nothing cleaned up PVs when an environment was deleted.

## What the rewrite changed

- **Safer deletes.** Only Radix-labelled objects for the app, namespace and component are deleted, while the old cluster-wide PV sweep could delete PVs that belonged to other namespaces. Deletes got `uid` preconditions.
- **Removal of the deprecated `type` volume mount.** The `type`, `storage`, `uid`, `gid`, `accessMode`, `requestsStorage` and `container` fields were removed from the CRD schemas, together with the `azure-blob` PV builder. [#2035](https://github.com/equinor/radix-operator/issues/2035) confirmed that no RadixApplication or RadixDeployment in any cluster used `type: azure-blob`, so no existing volume mount was affected.
- **No extra configuration or RBAC** was needed for the rewrite.

| Step                                  | Before                                                                                                              | After                                                                                     |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Build Pod volumes                     | `GetVolumes`: reuse the claim name of the current workload's volume with the same name, else generate a random name | `ReconcileComponentVolumes` / `EnsureComponentVolumes`: claim chosen by matching the spec |
| Build container mounts                | Separately, with `GetRadixDeployComponentVolumeMounts` (deployment and batch)                                       | Returned together with the volumes                                                        |
| Create PV/PVC                         | `CreateOrUpdatePVCVolumeResourcesForDeployComponent`, a second pass that could rename claims                        | In the same call, only when no current PV/PVC matches the spec, with new unique names     |
| Delete PV/PVC in RadixDeployment sync | `GarbageCollectCsiAzureVolumeResourcesForDeployComponent` at the end of **every** component's sync, namespace-wide  | Per component, plus garbage collection of removed components and jobs                     |

## RadixDeployment sync: before

```mermaid
flowchart TD
    A["OnSync"] --> B{"Inactive, or not<br/>the latest RD?"}
    B -->|yes| Z(["return"])
    B -->|no| D["syncDeployment"]
    D --> E["GC removed components and<br/>auxiliary resources, RBAC"]
    E --> F["syncSecrets<br/>(volume mount credential secrets)"]
    F --> G["Network policies, gateway"]
    G --> L{{"For each component and job scheduler"}}

    subgraph comp ["syncDeploymentForRadixComponent (one component)"]
        direction TB
        C1["ConfigMaps, ServiceAccount"]
        C2["setDesiredDeploymentProperties"]
        C3["GetVolumes<br/>claimName = current Deployment's claim with the same volume name,<br/>else pvc-&lt;volume&gt;-&lt;random5&gt;"]:::vol
        C4["GetRadixDeployComponentVolumeMounts"]:::vol
        C5["CreateOrUpdatePVCVolumeResourcesForDeployComponent<br/>find PV by claimRef name, compare;<br/>on mismatch: new random PVC + pv-&lt;uuid&gt;, rename claim"]:::vol
        C6["handleJobAuxDeployment<br/>(job scheduler: volumes moved to aux Deployment)"]
        C7["ApplyDeployment"]
        C8["Scaler, Service, PDB, HTTPRoute, ServiceMonitor"]
        C9["GarbageCollectCsiAzureVolumeResourcesForDeployComponent<br/>delete every PVC in the namespace not referenced by a<br/>Deployment/Job template, their PVs, and Released/Failed<br/>Blob PVs cluster-wide"]:::danger
        C1 --> C2 --> C3 --> C4 --> C5 --> C6 --> C7 --> C8 --> C9
    end

    L --> C1
    C9 -->|next component| L
    L -->|done| M{"Any component failed?"}
    M -->|yes| ERR["syncStatus: Failed"]
    M -->|no| N["External DNS, auxiliary resources"] --> S["syncStatus"]

    classDef vol fill:#dbeafe,stroke:#1d4ed8
    classDef danger fill:#fee2e2,stroke:#b91c1c
```

Problems in this loop:

- **GC ran N times per sync** (once per component), in the middle of the loop. Later components had not yet created their new PVCs or applied their Deployments.
- **The claim was chosen in two places.** `GetVolumes` picked a name, then `CreateOrUpdatePVCVolumeResourcesForDeployComponent` could replace it with a new random name.
- **GC only knew about Deployments and Jobs that already existed.** It ignored RadixBatches, Pods and specs, so a PVC that was just created for a batch could be deleted.

## RadixBatch sync: before

```mermaid
flowchart TD
    A["OnSync"] --> B{"Done and no<br/>restart requested?"}
    B -->|yes| Z(["return"])
    B -->|no| R["reconcile"]
    R --> R1["Get the referenced RD and job component"]
    R1 --> R2["List batch Jobs and Services"]
    R2 --> R3["Prepare lazy actualVolumesGetter<br/>(sync.OnceValues)"]:::vol
    R3 --> L{{"For each batch job"}}

    subgraph job ["reconcileKubeJob (one batch job)"]
        direction TB
        J0{"Stop requested?"}
        J1{"Done, or Kubernetes Job exists,<br/>and no restart?"}
        J2["Delete old Jobs on restart,<br/>validate payload secret"]
        J3["actualVolumesGetter() — first call only:<br/>GetExistingJobAuxComponentVolumes<br/>→ GetVolumes (aux claim or random name)<br/>→ CreateOrUpdatePVCVolumeResourcesForDeployComponent"]:::vol
        J4["buildJob<br/>mounts from GetRadixDeployComponentVolumeMounts + payload"]:::vol
        J5["Create Kubernetes Job"]
        J0 -->|yes| JD["Delete Jobs"]
        J0 -->|no| J1
        J1 -->|yes| JS(["skip, no volume work"])
        J1 -->|no| J2 --> J3 --> J4 --> J5
    end

    L --> J0
    L -->|done| S["syncStatus"]
    S --> E(["return — no volume GC"])

    classDef vol fill:#dbeafe,stroke:#1d4ed8
```

## How the two controllers interacted

The controllers created pairs with random names on their own, and the RadixDeployment GC could delete a pair the batch syncer had just created.

```mermaid
sequenceDiagram
    autonumber
    participant B as RadixBatch syncer
    participant K as Kubernetes API
    participant D as RadixDeployment syncer

    B->>K: Create PV pv-UUID1 + PVC pvc-x-abcde
    D->>K: Create PV pv-UUID2 + PVC pvc-x-fghij (same spec, duplicate)
    D->>K: GC: list Deployments/Jobs — pvc-x-abcde is not referenced yet
    D->>K: Delete pvc-x-abcde and pv-UUID1
    B->>K: Create Job with claimName pvc-x-abcde
    Note over B,K: The Pod stays Pending — its PVC is gone
```
