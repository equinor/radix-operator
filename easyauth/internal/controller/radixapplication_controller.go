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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/equinor/radix-operator/easyauth/internal/defaulter"
	"github.com/equinor/radix-operator/easyauth/internal/dex"
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
	// RestartPendingAnnotation is set on the OAuth2 proxy Secret when the client secret has changed, until the OAuth2 proxy is restarted
	RestartPendingAnnotation = "easyauth.radix.equinor.com/restart-pending"
	// RestartedAtAnnotation is set on the pod template of the OAuth2 proxy Deployment to restart it
	RestartedAtAnnotation = "easyauth.radix.equinor.com/restartedAt"

	// DefaultResyncInterval is how often each RadixApplication is reconciled to correct drift
	DefaultResyncInterval = time.Hour
	// DefaultSecretRotationInterval is how old a client secret can be before it is rotated, when not set
	DefaultSecretRotationInterval = 90 * 24 * time.Hour
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
	// Annotation that enables EasyAuth on a RadixApplication
	annotation string
	// OIDC issuer URL set on the OAuth2 configuration of public components
	oauth2IssuerURL string
	// Cluster name, DNS zone and app alias base URL of radix-operator, used to build the public hostnames of components
	clusterName     string
	dnsZone         string
	appAliasBaseURL string
	// How often each RadixApplication is reconciled to correct drift
	resyncInterval time.Duration
	// How old a client secret can be before it is rotated. Must be the same in all clusters sharing the Dex database.
	secretRotationInterval time.Duration
}

// NewRadixApplicationReconciler returns a reconciler that reads and writes Kubernetes objects with c, and manages
// the OAuth2 clients of applications in Dex with dexClients. secretRotationInterval defaults to
// DefaultSecretRotationInterval when not set.
func NewRadixApplicationReconciler(
	c client.Client,
	dexClients DexClientManager,
	annotation, oauth2IssuerURL, clusterName, dnsZone, appAliasBaseURL string,
	secretRotationInterval time.Duration,
) *RadixApplicationReconciler {
	if secretRotationInterval <= 0 {
		secretRotationInterval = DefaultSecretRotationInterval
	}
	return &RadixApplicationReconciler{
		Client:                 c,
		dexClients:             dexClients,
		annotation:             annotation,
		oauth2IssuerURL:        oauth2IssuerURL,
		clusterName:            clusterName,
		dnsZone:                dnsZone,
		appAliasBaseURL:        appAliasBaseURL,
		resyncInterval:         DefaultResyncInterval,
		secretRotationInterval: secretRotationInterval,
	}
}

// +kubebuilder:rbac:groups=radix.equinor.com,resources=radixapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=radix.equinor.com,resources=radixregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=radix.equinor.com,resources=radixdeployments,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=patch

// Reconcile ensures that RadixApplications with the EasyAuth annotation have the EasyAuth OAuth2 configuration on
// the public components of all their RadixDeployments, an OAuth2 client in Dex with redirect URIs for all public
// hosts, and that the client secret from Dex is synced to the OAuth2 proxy of each public component.
func (r *RadixApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Ctx(ctx).With().Str("controller", "radixapplication").Str("radixApplication", req.String()).Logger()
	ctx = logger.WithContext(ctx)

	ra := &radixv1.RadixApplication{}
	if err := r.Get(ctx, req.NamespacedName, ra); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !ra.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if _, ok := ra.GetAnnotations()[r.annotation]; !ok {
		return ctrl.Result{}, nil
	}

	clientID, err := dex.ClientIDForApp(ctx, r.Client, ra.Name)
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
	redirectURIs, err := r.redirectURIs(ctx, ra)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get redirect URIs: %w", err)
	}

	desired := dex.Client{ID: clientID, Name: ra.Name, RedirectURIs: redirectURIs}
	clientSecret, rotateAt, err := r.dexClients.EnsureClient(ctx, desired, r.secretRotationInterval)
	if err != nil {
		return time.Time{}, err
	}

	if err := r.syncComponentSecrets(ctx, ra, []byte(clientSecret)); err != nil {
		return time.Time{}, fmt.Errorf("failed to sync client secret to components: %w", err)
	}
	return rotateAt, nil
}

// redirectURIs returns the OAuth2 callback URL for every public host of the active RadixDeployment in each environment
// of the application. The hosts are the same as radix-operator creates HTTPRoutes for: the cluster, active cluster,
// app alias and external DNS hosts of each public component, and the DNS aliases of the component in the environment.
func (r *RadixApplicationReconciler) redirectURIs(ctx context.Context, ra *radixv1.RadixApplication) ([]string, error) {
	var uris []string
	for _, env := range ra.Spec.Environments {
		rd, err := r.activeRadixDeployment(ctx, utils.GetEnvironmentNamespace(ra.Name, env.Name))
		if err != nil {
			return nil, fmt.Errorf("environment %s: %w", env.Name, err)
		}
		if rd == nil {
			continue
		}
		for i := range rd.Spec.Components {
			component := &rd.Spec.Components[i]
			if !component.IsPublic() {
				continue
			}
			var hosts []string
			hosts = append(hosts, radixdeployment.GetComponentFQDNs(component, *rd, r.clusterName, r.dnsZone, r.appAliasBaseURL)...)
			for _, alias := range ra.Spec.DNSAlias {
				if alias.Environment == env.Name && alias.Component == component.Name {
					hosts = append(hosts, dnsalias.GetHostName(alias.Alias, r.dnsZone))
				}
			}
			for _, host := range hosts {
				uris = append(uris, dex.RedirectURI(host))
			}
		}
	}
	slices.Sort(uris)
	return slices.Compact(uris), nil
}

// activeRadixDeployment returns the active RadixDeployment in namespace, or nil if there is none
func (r *RadixApplicationReconciler) activeRadixDeployment(ctx context.Context, namespace string) (*radixv1.RadixDeployment, error) {
	rdList := &radixv1.RadixDeploymentList{}
	if err := r.List(ctx, rdList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	for i := range rdList.Items {
		if rdList.Items[i].IsActive() {
			return &rdList.Items[i], nil
		}
	}
	return nil, nil
}

// syncComponentSecrets writes the client secret to the OAuth2 proxy Secret of each public component in each environment.
// The OAuth2 proxy Secret is created by radix-operator when a component is deployed, and is skipped until it exists.
func (r *RadixApplicationReconciler) syncComponentSecrets(ctx context.Context, ra *radixv1.RadixApplication, clientSecret []byte) error {
	var errs []error
	for _, env := range ra.Spec.Environments {
		namespace := utils.GetEnvironmentNamespace(ra.Name, env.Name)
		for i := range ra.Spec.Components {
			component := &ra.Spec.Components[i]
			if !defaulter.IsPublic(component) {
				continue
			}
			if err := r.syncComponentSecret(ctx, namespace, component.Name, clientSecret); err != nil {
				errs = append(errs, fmt.Errorf("component %s in environment %s: %w", component.Name, env.Name, err))
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
			secret.Annotations[RestartPendingAnnotation] = "true"
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

	if _, ok := secret.Annotations[RestartPendingAnnotation]; !ok {
		return nil
	}
	if err := r.restartOAuthProxy(ctx, namespace, componentName); err != nil {
		return fmt.Errorf("failed to restart OAuth2 proxy: %w", err)
	}
	original := secret.DeepCopy()
	delete(secret.Annotations, RestartPendingAnnotation)
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
			RestartedAtAnnotation: time.Now().UTC().Format(time.RFC3339),
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

// syncRadixDeployments sets the EasyAuth OAuth2 config on public components of all RadixDeployments in the
// environments of the application. This covers RadixDeployments created before the annotation was added,
// and keeps inactive RadixDeployments correct in case they are promoted later.
func (r *RadixApplicationReconciler) syncRadixDeployments(ctx context.Context, ra *radixv1.RadixApplication, clientID string) error {
	desired := defaulter.OAuth2Config(clientID, r.oauth2IssuerURL)

	var errs []error
	for _, env := range ra.Spec.Environments {
		rdList := &radixv1.RadixDeploymentList{}
		if err := r.List(ctx, rdList, client.InNamespace(utils.GetEnvironmentNamespace(ra.Name, env.Name))); err != nil {
			errs = append(errs, fmt.Errorf("environment %s: %w", env.Name, err))
			continue
		}
		for i := range rdList.Items {
			if err := r.syncRadixDeployment(ctx, &rdList.Items[i], desired); err != nil {
				errs = append(errs, fmt.Errorf("RadixDeployment %s/%s: %w", rdList.Items[i].Namespace, rdList.Items[i].Name, err))
			}
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
	for i := range rd.Spec.Components {
		component := &rd.Spec.Components[i]
		if !isPublicDeployComponent(component) {
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

// isPublicDeployComponent mirrors how the operator determines if a deployed component is exposed to the internet
func isPublicDeployComponent(component *radixv1.RadixDeployComponent) bool {
	//nolint:staticcheck
	return component.PublicPort != "" || component.Public
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
