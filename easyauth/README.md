# Radix EasyAuth

Radix EasyAuth gives Radix applications authentication without any setup by the application team. When a
`RadixApplication` has the `radix.equinor.com/easyauth` annotation, every public component is put behind the
Radix OAuth2 proxy, which authenticates users against a Radix managed [Dex](https://dexidp.io/docs/) instance.

```yaml
apiVersion: radix.equinor.com/v1
kind: RadixApplication
metadata:
  name: my-app
  annotations:
    radix.equinor.com/easyauth: ""
```

EasyAuth runs as a separate deployment, installed with the [radix-easyauth](../charts/radix-easyauth) Helm chart.
It consists of:

| Part               | Status      | Responsibility                                                                                    |
| ------------------ | ----------- | ------------------------------------------------------------------------------------------------- |
| Mutating webhook   | Implemented | Overrides the OAuth2 configuration of public components                                           |
| Validating webhook | Implemented | Optionally rejects `RadixApplications` without the annotation                                     |
| Controller         | Planned     | Keeps an OAuth2 client in Dex and the client secret in Radix in sync with each `RadixApplication` |

## Mutating webhook

Runs on create and update of `RadixApplications` with the annotation. For each public component (`publicPort`
set, or the deprecated `public: true`) it:

- Replaces `authentication.oauth2` with the EasyAuth configuration below.
- Removes `authentication.oauth2` from all `environmentConfig` entries, since environment configuration is merged
  on top of the component configuration by radix-operator.

Any OAuth2 configuration set by the user on a public component is overridden. Private components and jobs are not
changed, and `RadixApplications` without the annotation are left untouched.

```yaml
authentication:
  oauth2:
    clientId: <app name>.dev.radix.equinor.com
    credentials: secret
    scope: openid profile email offline_access
    setAuthorizationHeader: false
    setXAuthRequestHeaders: false
    oidc:
      issuerUrl: <config.issuerUrl>
```

The webhook only rewrites the `RadixApplication`, it has no side effects (`sideEffects: None`). Everything that
talks to Dex or creates secrets belongs in the controller.

The client ID is temporary. When the controller is implemented, the webhook will set the client ID described in
[Client ID](#client-id), which requires it to read the `RadixRegistration` of the application.

## Validating webhook

Only installed when `config.enforced` is `true`. Rejects create and update of `RadixApplications` that do not have
the annotation, making EasyAuth mandatory for all applications in the cluster.

## Dex integration (planned)

Dex needs one OAuth2 client per application, with redirect URIs for every public host of the application. The
EasyAuth controller keeps these clients in sync using the [Dex gRPC API](https://dexidp.io/docs/configuration/api/).

### Why the gRPC API and not `staticClients`

- Dex reads `staticClients` from its config file at startup only, so every change requires a restart. During a
  rolling restart, Dex replicas disagree about which clients exist, and new applications can't log in until the
  next restart. Debouncing restarts reduces the number of restarts, but not these problems.
- Dex documents the gRPC API as the interface for management applications, and `staticClients` as a developer
  convenience.
- Clients created through the API are stored in Dex's database, which is a persistent database outside the
  cluster. They survive Dex restarts and are shared by all Dex replicas. Static clients are never stored in the
  database.
- Client secrets live in the Dex database and per application Secrets, instead of in one shared config object.

### Client ID

Each client is identified by the AppID of the application's `RadixRegistration` (`spec.appID`):

```text
easyauth.<appID>
```

The AppID is used instead of the application name because:

- **It is never reused.** An application name becomes available again when the application is deleted. With
  name based IDs, a new application with the same name would take over the old client, including its secret and
  redirect URIs, until cleanup removes it. With the AppID, a recreated application gets a new client, and the old
  client is an orphan that is harmless until it is removed. This is what makes it safe to clean up periodically
  instead of using a finalizer.
- **It is immutable** (enforced by the CRD) and unique across `RadixRegistrations` (enforced by radix-webhook), so
  a client ID never changes or collides during the lifetime of an application.

The AppID is a ULID and doesn't tell a human which application a client belongs to, so the controller sets the
Dex client `name` to the application name. Dex shows the name on its approval screen, and it makes clients easy
to identify when debugging.

`spec.appID` is optional in the `RadixRegistration` CRD. All existing `RadixRegistrations` have one, and the
API server sets it on new registrations. The controller and webhook treat a missing AppID as an error for that
application instead of falling back to the name. A missing AppID can be added later, since the CRD immutability
rule only applies when the old object has a value.

### Reconcile loop

For each `RadixApplication` with the annotation, the controller:

1. Reads the AppID from the `RadixRegistration` with the same name.
2. Builds the desired client: the [client ID](#client-id), the application name as `name`, and a redirect URI
   `https://<host>/oauth2/callback` for every public host of every public component in every environment.
3. Calls `GetClient`:
   - `not_found`: generates a secret and calls `CreateClient`.
   - Found: calls `UpdateClient` if redirect URIs are missing or the name differs, and keeps the existing secret.
     Existing redirect URIs are kept, see [Multiple clusters](#multiple-clusters).
4. Writes the client secret from Dex to the OAuth2 client secret of each public component, so the OAuth2 proxy can
   authenticate against Dex.
5. Requeues periodically to correct drift, and retries with backoff if Dex is unavailable.

`RadixApplications` without the annotation are ignored by the reconcile loop. Removing clients is handled only by
the cleanup loop below.

### Cleanup loop

The controller does not use a finalizer. A periodic loop instead deletes clients that are no longer needed:

1. Calls `ListClients`, and keeps the clients with the `easyauth.` prefix.
2. Builds the set of AppIDs for `RadixApplications` that have the annotation, from the informer cache.
3. Deletes every client whose AppID is not in the set.

This covers a deleted `RadixApplication` or `RadixRegistration`, a removed annotation, and an application that
was deleted and recreated with the same name.

- **Ordering:** clients are listed before `RadixApplications` are read. A client is only created after its
  `RadixApplication` is in the cache, so a client created during the loop is never deleted by mistake.
- **Failure handling:** the loop aborts without deleting anything if Dex or the cache returns an error, or if the
  cache has not synced. This prevents mass deletion when the controller's view of the cluster is incomplete.
- **Leader election:** the loop only runs on the leader.
- **Trade-off:** a client outlives its application until the next run. The orphaned client can't be used to log
  in, since its secret was only stored in the deleted application's namespaces and its ID is never reused.

### Multiple clusters

Several Radix clusters may write to the same Dex database, for example the active and passive cluster during a
cluster migration. `RadixRegistrations` keep their AppID when they are moved to another cluster, so all clusters
running an application share one client. The controller must therefore:

- **Treat Dex as the source of truth for the secret.** It only generates a secret when the client doesn't exist,
  and otherwise reads it from Dex. Clusters that generate their own secrets would keep overwriting each other.
  This also restores a lost or recreated Kubernetes Secret from Dex.
- **Merge redirect URIs instead of replacing them,** since each cluster has its own hostnames. The reconcile loop
  adds missing redirect URIs, but never removes URIs, since it can't tell if another cluster still uses them.
  Stale redirect URIs, for example from a removed component, are removed when the client is deleted.
- **Only run the cleanup loop in one cluster.** A cluster can't see which applications exist in other clusters,
  so it could delete a client another cluster still uses. The cleanup loop is enabled with `cleanupEnabled`,
  which should only be `true` in the active cluster. If it deletes a client used by an application that only
  exists in another cluster, that cluster's reconcile loop recreates the client with a new secret, and logins for
  that application fail until it has done so.

### Dex API constraints

- `UpdateClient` can't change the secret. Rotating a secret requires `DeleteClient` followed by `CreateClient`.
- Dex clients have no labels or metadata, so ownership is derived from the client ID prefix.
- The API has no authentication other than TLS client certificates. Dex's gRPC port must require a client
  certificate (`tlsClientCA`), issued to EasyAuth by cert-manager, and should be limited to EasyAuth with a
  NetworkPolicy.

## Configuration

Configured with the `config` section in the Helm chart [values](../charts/radix-easyauth/values.yaml), which is
rendered into a ConfigMap and parsed into [EasyAuthConfig](config.go).

| Value                       | Description                                              |
| --------------------------- | -------------------------------------------------------- |
| `annotation`                | Annotation that enables EasyAuth on a `RadixApplication` |
| `enforced`                  | Reject `RadixApplications` without the annotation        |
| `issuerUrl`                 | OIDC issuer URL of Dex, set on all public components     |
| `cleanupEnabled` (planned)  | Run the [cleanup loop](#cleanup-loop) in this cluster    |
| `cleanupInterval` (planned) | How often the [cleanup loop](#cleanup-loop) runs         |

The webhook serving certificate is issued by cert-manager, which also injects the CA bundle into the webhook
configurations.

## Testing

- Unit tests: `go test ./easyauth/...`
- End-to-end tests: `e2e/radix_easyauth_test.go`, see the [e2e README](../e2e/README.md). The e2e setup installs
  cert-manager and the radix-easyauth chart in the kind cluster.

## References

- Example Dex config: https://github.com/dexidp/dex/blob/master/examples/config-dev.yaml
- Dex gRPC API: https://dexidp.io/docs/configuration/api/
- Dex API definition: https://github.com/dexidp/dex/blob/master/api/v2/api.proto
- Dex storage: https://dexidp.io/docs/configuration/storage/
- https://github.com/dexidp/dex
- https://dexidp.io/docs/
