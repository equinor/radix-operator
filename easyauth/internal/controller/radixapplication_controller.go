package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/equinor/radix-operator/easyauth/internal/annotations"
	"github.com/equinor/radix-operator/easyauth/internal/dex"
	"github.com/equinor/radix-operator/easyauth/internal/oauth"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixdeployment "github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
)

const (
	// ClientSecretKey is the key in the OAuth2 proxy Secret that holds the OAuth2 client secret
	ClientSecretKey = defaults.OAuthClientSecretKeyName

	// DefaultResyncInterval is how often each RadixApplication is reconciled to correct drift
	DefaultResyncInterval = time.Hour
	// DefaultSecretRotationInterval is how old a client secret can be before it is rotated, when not set
	DefaultSecretRotationInterval = 90 * 24 * time.Hour
	// SyncTimeout is how long the sync of one RadixApplication can take. The sync is not cancelled on shutdown, so the
	// manager's GracefulShutdownTimeout must be longer than this.
	SyncTimeout = 30 * time.Second
)

// DexClientManager ensures that the OAuth2 client of an application exists in Dex
type DexClientManager interface {
	// EnsureClient creates or updates the client, rotates its secret when it is older than secretRotationInterval,
	// and returns the secret and when it must be rotated
	EnsureClient(ctx context.Context, desired dex.Client, secretRotationInterval time.Duration) (secret string, rotateAt time.Time, err error)
}

var _ DexClientManager = (*dex.ClientManager)(nil)

// RadixApplicationReconciler reconciles a RadixApplication object
type RadixApplicationReconciler struct {
	client.Client
	dexClients DexClientManager
	// OAuth2 configuration of public components, without the client ID (see oauth.NewTemplate)
	oauth2Template *radixv1.OAuth2
	// Cluster name, DNS zone and app alias base URL of radix-operator, used to build the public hostnames of components
	clusterName     string
	dnsZone         string
	appAliasBaseURL string
	// How often each RadixApplication is reconciled to correct drift
	resyncInterval time.Duration
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles for this controller
	MaxConcurrentReconciles int
	// How old a client secret can be before it is rotated. Must be the same in all clusters sharing the Dex database.
	secretRotationInterval time.Duration
}

// NewRadixApplicationReconciler returns a reconciler that reads and writes Kubernetes objects with c, and manages
// the OAuth2 clients of applications in Dex with dexClients. secretRotationInterval defaults to
// DefaultSecretRotationInterval when not set.
func NewRadixApplicationReconciler(
	c client.Client,
	dexClients DexClientManager,
	oauth2Template *radixv1.OAuth2,
	clusterName, dnsZone, appAliasBaseURL string,
	secretRotationInterval time.Duration,
	maxConcurrentReconciles int,
) *RadixApplicationReconciler {
	if secretRotationInterval <= 0 {
		secretRotationInterval = DefaultSecretRotationInterval
	}
	return &RadixApplicationReconciler{
		Client:                  c,
		dexClients:              dexClients,
		oauth2Template:          oauth2Template,
		clusterName:             clusterName,
		dnsZone:                 dnsZone,
		appAliasBaseURL:         appAliasBaseURL,
		resyncInterval:          DefaultResyncInterval,
		secretRotationInterval:  secretRotationInterval,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}
}

// Reconcile ensures that RadixApplications with the EasyAuth annotation have the EasyAuth OAuth2 configuration on
// the public components of all their RadixDeployments, an OAuth2 client in Dex with redirect URIs for all public
// hosts, and that the client secret from Dex is synced to the OAuth2 proxy of each public component.
//
// The manager cancels ctx on shutdown, so the sync runs on a context that is not cancelled with it, and only bounded
// by SyncTimeout. A started sync is completed before the manager releases the leader lease, so the Dex client and
// the client secret in the OAuth2 proxy Secrets are never left half-synced by a shutdown.
func (r *RadixApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), SyncTimeout)
	defer cancel()

	logger := log.Ctx(ctx).With().Str("controller", "radixapplication").Str("radixApplication", req.String()).Logger()
	ctx = logger.WithContext(ctx)

	ra := &radixv1.RadixApplication{}
	if err := r.Get(ctx, req.NamespacedName, ra); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !ra.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if _, ok := ra.GetAnnotations()[annotations.EasyAuth]; !ok {
		return ctrl.Result{}, nil
	}

	clientID, err := oauth.ClientIDForApp(ctx, r.Client, ra.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// RadixDeployments are synced first, since they only need the client ID. A public component without the EasyAuth
	// OAuth2 config is exposed without authentication, so this must not wait for Dex. With the OAuth2 config, but
	// before the client secret is synced, the OAuth2 proxy can't start and the component is unreachable instead.
	// A failure doesn't stop the Dex sync, and is returned with its errors.
	var rdErr error
	if err := r.syncRadixDeployments(ctx, ra, clientID); err != nil {
		rdErr = fmt.Errorf("failed to sync OAuth2 config to RadixDeployments: %w", err)
	}

	rotateAt, err := r.syncDexClient(ctx, ra, clientID)
	if err := errors.Join(rdErr, err); err != nil {
		return ctrl.Result{}, err
	}

	logger.Debug().Str("clientId", clientID).Msg("Reconciled RadixApplication")
	return ctrl.Result{RequeueAfter: r.requeueAfter(rotateAt)}, nil
}

// syncDexClient ensures the Dex client of the application, and syncs its secret to the OAuth2 proxy of each public
// component. It returns when the client secret must be rotated.
func (r *RadixApplicationReconciler) syncDexClient(ctx context.Context, ra *radixv1.RadixApplication, clientID string) (time.Time, error) {
	deployments, err := r.activeRadixDeployments(ctx, ra)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get active RadixDeployments: %w", err)
	}

	desired := dex.Client{ID: clientID, Name: ra.Name, RedirectURIs: r.redirectURIs(ra, deployments)}
	clientSecret, rotateAt, err := r.dexClients.EnsureClient(ctx, desired, r.secretRotationInterval)
	if err != nil {
		return time.Time{}, err
	}

	if err := r.syncComponentSecrets(ctx, deployments, []byte(clientSecret)); err != nil {
		return time.Time{}, fmt.Errorf("failed to sync client secret to components: %w", err)
	}
	return rotateAt, nil
}

// environmentDeployment is the active RadixDeployment of an environment
type environmentDeployment struct {
	env string
	rd  *radixv1.RadixDeployment
}

// activeRadixDeployments returns the active RadixDeployments of the application, including orphaned environments
func (r *RadixApplicationReconciler) activeRadixDeployments(ctx context.Context, ra *radixv1.RadixApplication) ([]environmentDeployment, error) {
	rdList := &radixv1.RadixDeploymentList{}
	if err := r.List(ctx, rdList, client.MatchingLabels{kube.RadixAppLabel: ra.Name}); err != nil {
		return nil, err
	}
	var deployments []environmentDeployment
	for i := range rdList.Items {
		rd := &rdList.Items[i]
		if !rd.IsActive() {
			continue
		}

		env, ok := rd.Labels[kube.RadixEnvLabel]
		if !ok || env == "" {
			return nil, fmt.Errorf("RadixDeployment %s/%s is missing environment label for app %s", rd.Namespace, rd.Name, ra.Name)
		}

		deployments = append(deployments, environmentDeployment{env: env, rd: rd})
	}
	return deployments, nil
}

// redirectURIs returns callback URLs for public hosts in active RadixDeployments and aliases from the latest
// RadixApplication. The callback path is built from the proxy prefix of the EasyAuth OAuth2 config.
func (r *RadixApplicationReconciler) redirectURIs(ra *radixv1.RadixApplication, deployments []environmentDeployment) []string {
	var uris []string
	for _, d := range deployments {
		for _, component := range d.rd.Spec.Components {
			if !component.IsPublic() {
				continue
			}
			var hosts []string
			hosts = append(hosts, radixdeployment.GetComponentFQDNs(&component, *d.rd, r.clusterName, r.dnsZone, r.appAliasBaseURL)...)
			for _, alias := range ra.Spec.DNSAlias {
				if alias.Environment == d.env && alias.Component == component.Name {
					hosts = append(hosts, dnsalias.GetHostName(alias.Alias, r.dnsZone))
				}
			}
			for _, host := range hosts {
				uris = append(uris, oauth.RedirectURI(host, r.oauth2Template.ProxyPrefix))
			}
		}
	}
	slices.Sort(uris)
	return slices.Compact(uris)
}

// syncComponentSecrets writes the client secret to the OAuth2 proxy Secret of each public component of the active
// RadixDeployment in each environment. These are the components that run an OAuth2 proxy with the EasyAuth config,
// which may differ from the latest RadixApplication until it is deployed.
// The OAuth2 proxy Secret is created by radix-operator when a component is deployed, and is skipped until it exists.
func (r *RadixApplicationReconciler) syncComponentSecrets(ctx context.Context, deployments []environmentDeployment, clientSecret []byte) error {
	var errs []error
	for _, d := range deployments {
		for _, component := range d.rd.Spec.Components {
			if !component.IsPublic() {
				continue
			}
			if err := r.syncComponentSecret(ctx, d.rd.Namespace, component.Name, clientSecret); err != nil {
				errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, d.env, err))
			}
		}
	}
	return errors.Join(errs...)
}

// syncComponentSecret writes the client secret to the OAuth2 proxy Secret of a component, and restarts the OAuth2 proxy
// when an existing client secret is changed, since the OAuth2 proxy only reads it at startup.
// A missing client secret doesn't need a restart, since the OAuth2 proxy container can't start without it.
func (r *RadixApplicationReconciler) syncComponentSecret(ctx context.Context, namespace, componentName string, clientSecret []byte) error {
	name := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return client.IgnoreNotFound(err)
	}

	if !bytes.Equal(secret.Data[ClientSecretKey], clientSecret) {
		original := secret.DeepCopy()
		// The pending restart is stored with the new value, so it is retried if the restart fails
		if len(secret.Data[ClientSecretKey]) > 0 {
			if secret.Annotations == nil {
				secret.Annotations = map[string]string{}
			}
			secret.Annotations[annotations.RestartPending] = "true"
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data[ClientSecretKey] = clientSecret
		if err := r.Patch(ctx, secret, client.MergeFrom(original)); err != nil {
			return err
		}
		log.Ctx(ctx).Info().Str("namespace", namespace).Str("secret", name).Msg("Synced client secret")
	}

	if _, ok := secret.Annotations[annotations.RestartPending]; !ok {
		return nil
	}
	if err := r.restartOAuthProxy(ctx, namespace, componentName); err != nil {
		return fmt.Errorf("failed to restart OAuth2 proxy: %w", err)
	}
	original := secret.DeepCopy()
	delete(secret.Annotations, annotations.RestartPending)
	return r.Patch(ctx, secret, client.MergeFrom(original))
}

// restartOAuthProxy triggers a rollout of the OAuth2 proxy Deployment of a component, the same way as kubectl rollout restart.
// radix-operator removes the annotation the next time it syncs the RadixDeployment, which causes one more rollout.
func (r *RadixApplicationReconciler) restartOAuthProxy(ctx context.Context, namespace, componentName string) error {
	deployment := &appsv1.Deployment{
		Name:      utils.GetAuxiliaryComponentDeploymentName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix),
		Namespace: namespace,
	}
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": map[string]string{
			annotations.RestartedAt: time.Now().UTC().Format(time.RFC3339),
		}}}},
	})
	if err != nil {
		return err
	}
	// Patched without reading it first, so Deployments are not cached. A missing Deployment is created by
	// radix-operator with the new client secret, and doesn't need a restart.
	if err := r.Patch(ctx, deployment, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return client.IgnoreNotFound(err)
	}
	log.Ctx(ctx).Info().Str("namespace", namespace).Str("deployment", deployment.Name).Msg("Restarted OAuth2 proxy")
	return nil
}

// syncRadixDeployments sets the EasyAuth OAuth2 config on public components of the active RadixDeployments in the
// environments of the application. This covers RadixDeployments created before the annotation was added.
// Inactive RadixDeployments are skipped. A promoted RadixDeployment is synced when it becomes active.
// RadixDeployments are not changed when the RadixApplication has no allowed groups, for example when the default
// allowed identities are removed from the EasyAuth config.
func (r *RadixApplicationReconciler) syncRadixDeployments(ctx context.Context, ra *radixv1.RadixApplication, clientID string) error {
	desired := oauth.ForApp(r.oauth2Template, clientID, oauth.SplitIdentities(ra.GetAnnotations()[annotations.AllowedIdentities]))
	if len(desired.AllowedGroups) == 0 {
		return fmt.Errorf("annotation %q must be set, since no default allowed identities are configured", annotations.AllowedIdentities)
	}

	rdList := &radixv1.RadixDeploymentList{}
	if err := r.List(ctx, rdList, client.MatchingLabels{kube.RadixAppLabel: ra.Name}); err != nil {
		return fmt.Errorf("failed to list radix deployments for app %s: %w", ra.Name, err)
	}

	var errs []error
	for _, item := range rdList.Items {
		if item.Status.Condition != radixv1.DeploymentActive {
			continue
		}

		if err := r.syncRadixDeployment(ctx, &item, desired); err != nil {
			errs = append(errs, fmt.Errorf("RadixDeployment %s/%s: %w", item.Namespace, item.Name, err))
		}
	}

	return errors.Join(errs...)
}

// syncRadixDeployment patches rd with the desired OAuth2 config. On a conflict, the RadixDeployment is read again and
// the patch is retried with exponential backoff.
func (r *RadixApplicationReconciler) syncRadixDeployment(ctx context.Context, rd *radixv1.RadixDeployment, desired *radixv1.OAuth2) error {
	firstAttempt := true
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if !firstAttempt {
			if err := r.Get(ctx, client.ObjectKeyFromObject(rd), rd); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		firstAttempt = false
		return r.patchRadixDeployment(ctx, rd, desired)
	})
}

func (r *RadixApplicationReconciler) patchRadixDeployment(ctx context.Context, rd *radixv1.RadixDeployment, desired *radixv1.OAuth2) error {
	original := rd.DeepCopy()
	changed := false

	if _, ok := rd.Annotations[annotations.EasyAuth]; !ok {
		if rd.Annotations == nil {
			rd.Annotations = map[string]string{}
		}
		rd.Annotations[annotations.EasyAuth] = ""
		changed = true
	}

	for i := range rd.Spec.Components {
		component := &rd.Spec.Components[i] // Keep a reference to the original, 'for _, comp := ...' makes a copy
		if !component.IsPublic() {
			continue
		}
		if component.Authentication != nil && equality.Semantic.DeepEqual(component.Authentication.OAuth2, desired) {
			continue
		}
		if component.Authentication == nil {
			component.Authentication = &radixv1.Authentication{}
		}
		component.Authentication.OAuth2 = desired.DeepCopy()
		changed = true
	}
	if !changed {
		return nil
	}

	// Components is a list, so a merge patch replaces all components. The optimistic lock prevents overwriting concurrent changes.
	if err := r.Patch(ctx, rd, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	log.Ctx(ctx).Info().Str("namespace", rd.Namespace).Str("radixDeployment", rd.Name).Msg("Updated OAuth2 config")
	return nil
}

// requeueAfter returns the resync interval, or the time until the client secret must be rotated if that is sooner.
// Every cluster sharing the Dex database requeues at the rotation time, so they get the new secret right after it is
// rotated. A random delay of up to a minute spreads the clusters, so they are less likely to rotate at the same time.
func (r *RadixApplicationReconciler) requeueAfter(rotateAt time.Time) time.Duration {
	untilRotation := max(time.Until(rotateAt), 0) + rand.N(time.Minute)
	return min(r.resyncInterval, untilRotation)
}

// mapToRadixApplication maps objects with the radix-app label to the RadixApplication of the app
func mapToRadixApplication(_ context.Context, obj client.Object) []reconcile.Request {
	appName := obj.GetLabels()[kube.RadixAppLabel]
	if appName == "" {
		return nil
	}
	return []reconcile.Request{{Namespace: utils.GetAppNamespace(appName), Name: appName}}
}

// conditionChangedPredicate triggers when a RadixDeployment becomes active or inactive, which is a status change
// that doesn't change the generation
var conditionChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldRD, okOld := e.ObjectOld.(*radixv1.RadixDeployment)
		newRD, okNew := e.ObjectNew.(*radixv1.RadixDeployment)
		return okOld && okNew && oldRD.Status.Condition != newRD.Status.Condition
	},
}

func isOAuthProxySecret(obj client.Object) bool {
	return obj.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel] == radixv1.OAuthProxyAuxiliaryComponentType
}

// SetupWithManager sets up the controller with the Manager.
func (r *RadixApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&radixv1.RadixApplication{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles}).
		// The public hosts, and therefore the redirect URIs in Dex, come from the active RadixDeployment
		Watches(&radixv1.RadixDeployment{},
			handler.EnqueueRequestsFromMapFunc(mapToRadixApplication),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, conditionChangedPredicate)),
		).
		// The OAuth2 proxy Secret is created by radix-operator after the RadixDeployment, so it must be watched to sync the client secret
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(mapToRadixApplication),
			builder.WithPredicates(predicate.NewPredicateFuncs(isOAuthProxySecret)),
		).
		Named("radixapplication").
		Complete(r)
}
