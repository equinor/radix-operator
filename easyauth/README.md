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
The `ghcr.io/equinor/radix/easyauth` image and the `oci://ghcr.io/equinor/radix/charts/radix-easyauth` chart are
built in pull requests, and published with the same version as radix-operator on each release.
It consists of:

| Part               | Status      | Responsibility                                                                                   |
| ------------------ | ----------- | ------------------------------------------------------------------------------------------------ |
| Mutating webhook   | Implemented | Overrides the OAuth2 configuration of public components                                          |
| Validating webhook | Implemented | Optionally rejects `RadixApplications` without the annotation                                    |
| Dex                | Implemented | OIDC provider that users log in to, installed by the chart. Required                             |
| Controller         | Implemented | Keeps an OAuth2 client in Dex, the client secret and `RadixDeployments` in sync with each app    |
| Cleanup loop       | Implemented | Deletes Dex clients that no longer belong to an application, when `config.cleanupEnabled` is set |

The webhooks and the controller run in the same process and Deployment, so they always share configuration and
version. The webhooks are served by every replica, while the controller only runs on the leader. Since the webhooks
use `failurePolicy: Fail`, an unavailable EasyAuth blocks all creates and updates of `RadixApplications`. The chart
therefore defaults to 2 replicas, spread across nodes with a preferred pod anti-affinity (unless `affinity` is set),
and a PodDisruptionBudget with `minAvailable: 1` when `replicaCount` is greater than 1. The chart fails to render
if `config.enableLeaderElection` is `false` and `replicaCount` is greater than 1.

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
    clientId: easyauth.<appID>
    credentials: secret
    scope: openid profile email offline_access
    setAuthorizationHeader: false
    setXAuthRequestHeaders: false
    oidc:
      issuerUrl: <config.issuerUrl>
```

The webhook only rewrites the `RadixApplication`, it has no side effects (`sideEffects: None`). Everything that
talks to Dex or creates secrets belongs in the controller.

The client ID is described in [Client ID](#client-id). The webhook reads the `RadixRegistration` of the
application from the API server, not a cache, since it may have just been created. The request is rejected if the
`RadixRegistration` doesn't exist or has no AppID.

## Validating webhook

Only installed when `config.enforced` is `true`. Rejects create and update of `RadixApplications` that do not have
the annotation, making EasyAuth mandatory for all applications in the cluster.

## Controller

Reconciles `RadixApplications` with the annotation. `RadixApplications` without the annotation are ignored. For
each application it:

1. **Reads the AppID** from the `RadixRegistration` with the same name, and builds the [client ID](#client-id).
2. **Ensures the Dex client** with the [reconcile loop](#reconcile-loop), and gets the client secret from Dex.
3. **Syncs the client secret to public components.** For every public component in every environment, writes the
   client secret to the `ClientSecret` key of the OAuth2 proxy Secret (`<component>-aux-oauth-*`). The OAuth2 proxy
   Secret is created by radix-operator when the component is deployed, and is skipped until it exists. Other keys,
   like `CookieSecret`, are not changed.
4. **Syncs the OAuth2 config to `RadixDeployments`.** Sets the OAuth2 configuration from the
   [mutating webhook](#mutating-webhook) on the public components of every `RadixDeployment`, active and inactive,
   in the environments of the application. This covers `RadixDeployments` created before the annotation was added,
   and inactive `RadixDeployments` that may be promoted later. radix-operator redeploys active `RadixDeployments`
   when they change. The patch uses optimistic locking, and is retried on conflict.
5. Requeues every hour to correct drift. Errors, for example when Dex is unavailable, are retried with backoff.

Only Secrets and `RadixDeployments` in the environment namespaces of the application's `RadixApplication` are
changed. The controller also watches `RadixDeployments` (spec changes and activation) and OAuth2 proxy Secrets,
mapped to their application by the `radix-app` label, so new deployments and hosts are handled without waiting for
the next resync. Only OAuth2 proxy Secrets (label `radix-aux-component-type=oauth`) are cached.

The OAuth2 proxy reads the client secret at startup. When the controller changes an existing client secret, for
example after the Dex client was recreated or the value was changed by a user, it restarts the OAuth2 proxy:

1. Sets the annotation `easyauth.radix.equinor.com/restart-pending` on the OAuth2 proxy Secret, in the same patch as
   the new client secret, so a failed restart is retried by the next reconcile.
2. Sets `easyauth.radix.equinor.com/restartedAt` on the pod template of the OAuth2 proxy Deployment
   (`<component>-aux-oauth`), which rolls the pods like `kubectl rollout restart`. This is the same as restarting
   the OAuth2 proxy in the Radix API, which uses `radixapi/restartedAt`.
3. Removes `restart-pending` from the Secret.

The OAuth2 proxy is not restarted when the client secret is set for the first time, since its container can't
start until the key exists, and starts with the new value when it does. radix-operator doesn't know about the
`restartedAt` annotation and removes it the next time it syncs the `RadixDeployment`, which rolls the pods once more.
The Deployment is patched without being read, so Deployments are not cached and EasyAuth only needs the `patch`
permission on them.

## Dex

The chart always installs Dex, since EasyAuth stores OAuth2 clients in it:

| Resource                       | Purpose                                                                                            |
| ------------------------------ | -------------------------------------------------------------------------------------------------- |
| Deployment and Service         | Dex, with the HTTP (5556), gRPC API (5557) and telemetry (5558) ports                              |
| Secret `<fullname>-dex-config` | Dex configuration, rendered from `config.issuerUrl`, `dex.storage` and the free-form `dex.config`  |
| HTTPRoute                      | Exposes Dex on the host and path of `config.issuerUrl`, attached to `dex.httpRoute.parentRefs`     |
| Certificates                   | gRPC serving certificate for Dex, and a client certificate for EasyAuth                            |
| NetworkPolicy                  | Only EasyAuth pods can reach the gRPC port                                                         |
| ServiceAccount                 | Without a token or roles, since Dex stores its data in Postgres and doesn't use the Kubernetes API |

- **Issuer:** `config.issuerUrl` is both the Dex issuer and the issuer set on public components by the mutating
  webhook, so they always match.
- **Storage:** Dex stores clients, signing keys and sessions in Postgres (`dex.storage.postgres`). The password is
  read from an existing Secret and passed to Dex as `$DEX_POSTGRES_PASSWORD`, which Dex expands in its config.
- **Configuration:** connectors and other settings come from `dex.config`. `issuer`, `storage`, `grpc` and
  `telemetry` are always set by the chart, and `web.http` is always set, but other `web` settings are merged.
  Secrets for connectors can be added with `dex.extraEnv` and referenced as `$VAR` in `dex.config`.
- **Routing:** `dex.httpRoute.parentRefs` is passed to the HTTPRoute as is, so it can reference a `Gateway`, a
  `ListenerSet`, or both.
- **gRPC authentication:** the gRPC serving certificate and the EasyAuth client certificate are issued by the
  chart's CA, the same CA as the webhook serving certificate. Dex requires a client certificate signed by this CA
  (`tlsClientCA`), and only accepts certificates with the `client auth` usage, so the webhook serving certificate
  can't be used as a client certificate. The client certificate is mounted in EasyAuth, and its location and the
  Dex gRPC address are added to the EasyAuth config as `dex.certsDir` and `dex.grpcAddress`.
- **Image entrypoint:** the container runs the `dex` binary directly instead of the image entrypoint, which
  renders the config with gomplate into `/tmp` and doesn't work with a read-only root filesystem.

## Dex integration

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
   `https://<host>/oauth2/callback` for every public host of the active `RadixDeployment` in each environment of the
   application. The hosts are built the same way as radix-operator builds the hosts of its HTTPRoutes, from
   `config.clusterName`, `config.dnsZone` and `config.appAliasBaseURL`: the cluster, active cluster, app alias and
   external DNS hosts of each public component, and the `dnsAlias` entries of the `RadixApplication` for the
   component and environment. Since the hosts don't depend on the HTTPRoutes, redirect URIs are in Dex before the
   routes are ready.
3. Calls `GetClient`:
   - Not found: generates a secret and calls `CreateClient`. If the client was created concurrently, for example by
     another cluster, it is read again with `GetClient`. Dex returns a missing client as gRPC `Unknown` with the
     message `not found`, which is handled the same as `NotFound`.
   - Found: calls `UpdateClient` if redirect URIs are missing or the name differs, and keeps the existing secret.
     Existing redirect URIs are kept, see [Multiple clusters](#multiple-clusters). If the secret is due for
     rotation, the client is rotated instead, see [Secret rotation](#secret-rotation).
   - Other errors: nothing is created, and the reconcile is retried.
4. Writes the client secret from Dex to the OAuth2 client secret of each public component, so the OAuth2 proxy can
   authenticate against Dex.
5. Requeues periodically to correct drift, or at the secret rotation time if that is sooner, and retries with
   backoff if Dex is unavailable.

`RadixApplications` without the annotation are ignored by the reconcile loop. Removing clients is handled only by
the cleanup loop below.

### Secret rotation

Client secrets are rotated when they are older than `config.secretRotationInterval`, 90 days by default.

The rotation interval is owned by the controller, and passed to `EnsureClient` of the Dex client manager
(`internal/dex`), which rotates the secret when it is due and returns when the next rotation is due.

- **Creation time:** Dex clients have no metadata, so the creation time is stored in the secret itself. A secret is
  a ULID, which encodes the creation time, followed by `.` and 128 bits of random data from `crypto/rand`. Every
  cluster sharing the Dex database can tell when a secret must be rotated, without any state in Kubernetes. A secret
  without a creation time, for example from an older version, is rotated on the next reconcile.
- **Rotation:** `UpdateClient` can't change the secret, so the client is deleted and created again with the same ID
  and name, a new secret, and all existing redirect URIs, including the ones from other clusters. If creating fails,
  the next reconcile creates the client, and other clusters add their redirect URIs again.
- **Distribution:** the new secret is written to the OAuth2 proxy Secrets, and the OAuth2 proxies are
  [restarted](#controller), like any other changed client secret.
- **Timing:** the reconcile loop requeues at the rotation time, plus a random delay of up to a minute, if that is
  sooner than the hourly resync. Every cluster does this, so they get the new secret within about a minute.
- **Concurrent rotation:** all clusters rotate, since they all know the rotation time. The first cluster rotates the
  secret, and the others see a new secret that is not due for rotation. If two clusters rotate at the same time,
  `CreateClient` returns `AlreadyExists` for one of them, which then uses the secret created by the other. In the
  rare case where one cluster deletes the client right after another has created it, the secret is rotated twice,
  and the first cluster picks up the final secret on its next reconcile.
- **Disruption:** the old secret stops working as soon as the client is deleted. Until the OAuth2 proxies have
  restarted, logins and token refreshes fail. Existing sessions that don't need a refresh are not affected. Refresh
  tokens in Dex are kept, since the client ID doesn't change.

### Cleanup loop

The controller does not use a finalizer. A periodic loop instead deletes clients that are no longer needed:

1. Calls `ListClients`, and keeps the clients with the `easyauth.` prefix.
2. Builds the set of AppIDs for `RadixApplications` that have the annotation, from the informer cache. The AppID is
   read from the `RadixRegistration` with the same name.
3. Deletes every client whose AppID is not in the set.

The loop runs right after the cache has synced, and then every `config.cleanupInterval`.

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

- **Treat Dex as the source of truth for the secret.** It only generates a secret when the client doesn't exist or
  the secret is due for [rotation](#secret-rotation), and otherwise reads it from Dex. Clusters that generate their own secrets would keep overwriting each other.
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
- The API has no authentication other than TLS client certificates. The chart configures Dex to require a client
  certificate and limits the gRPC port to EasyAuth with a NetworkPolicy, see [Dex](#dex).

## Configuration

Configured with the `config` section in the Helm chart [values](../charts/radix-easyauth/values.yaml), which is
rendered into a ConfigMap and parsed into [EasyAuthConfig](config.go).

| Value                    | Description                                                                                                           |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| `annotation`             | Annotation that enables EasyAuth on a `RadixApplication`                                                              |
| `enforced`               | Reject `RadixApplications` without the annotation                                                                     |
| `issuerUrl`              | OIDC issuer URL of Dex, set on all public components                                                                  |
| `clusterName`            | Name of the cluster, same as `config.common.clusterName` of radix-operator. Required                                  |
| `dnsZone`                | DNS zone, same as `config.common.dnsZone` of radix-operator. Required                                                 |
| `appAliasBaseURL`        | App alias base URL, same as `config.common.appAliasBaseURL` of radix-operator. Required                               |
| `cleanupEnabled`         | Run the [cleanup loop](#cleanup-loop) in this cluster. Defaults to `false`                                            |
| `cleanupInterval`        | How often the [cleanup loop](#cleanup-loop) runs. At least `1m` when cleanup is enabled                               |
| `secretRotationInterval` | How old a client secret can be before it is [rotated](#secret-rotation). Defaults to `2160h` (90 days), at least `1h` |

`dex.grpcAddress` and `dex.certsDir` are added to the config by the chart, and are required.

The chart templates `mutating_webhook.yaml` and `validating_webhook.yaml` are generated from the `+kubebuilder:webhook`
markers in `internal/defaulter` and `internal/validator` by `make helmresources`, with
[helmify-easyauth-mutating-webhook.sh](../hack/helmify-easyauth-mutating-webhook.sh) and
[helmify-easyauth-validating-webhook.sh](../hack/helmify-easyauth-validating-webhook.sh) adding the Helm template
expressions. Do not edit them by hand.

The webhook serving certificate is issued by cert-manager, which also injects the CA bundle into the webhook
configurations.

Dex is configured with the `dex` section:

| Value                       | Description                                                               |
| --------------------------- | ------------------------------------------------------------------------- |
| `dex.storage.postgres`      | Postgres host, port, database, user, password Secret and SSL mode         |
| `dex.config`                | Free-form Dex configuration, for example connectors and `oauth2` settings |
| `dex.extraEnv`              | Additional environment variables, for example connector client secrets    |
| `dex.httpRoute.parentRefs`  | Gateways and/or ListenerSets the Dex HTTPRoute is attached to             |
| `dex.networkPolicy.enabled` | Limit the gRPC port to EasyAuth pods. Defaults to `true`                  |

## Testing

- Unit tests: `go test ./easyauth/...`. Dex is replaced by an in-memory fake (`internal/dex/dextest`), and the
  mTLS connection is tested against an in-process gRPC server.
- End-to-end tests: `e2e/radix_easyauth_test.go`, see the [e2e README](../e2e/README.md). The e2e setup installs
  cert-manager, a throwaway Postgres (`e2e/testdata/dex-postgres.yaml`) and the radix-easyauth chart with Dex in the
  kind cluster, and waits for Dex to be ready. The issuer is the in-cluster Dex Service, and Dex has a static
  password user. The pipeline test logs in as that user through port forwards to the OAuth2 proxy and Dex pods, and
  verifies that the web component is reached. It also verifies that unauthenticated requests, forged session cookies,
  wrong passwords, unknown users and cancelled approvals are rejected. Dex always shows the approval screen, since
  the OAuth2 proxy sends `approval_prompt=force`, which overrides `oauth2.skipApprovalScreen` in Dex.

## References

- Example Dex config: https://github.com/dexidp/dex/blob/master/examples/config-dev.yaml
- Dex gRPC API: https://dexidp.io/docs/configuration/api/
- Dex API definition: https://github.com/dexidp/dex/blob/master/api/v2/api.proto
- Dex storage: https://dexidp.io/docs/configuration/storage/
- https://github.com/dexidp/dex
- https://dexidp.io/docs/
