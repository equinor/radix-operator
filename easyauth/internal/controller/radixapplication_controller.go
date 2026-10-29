package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/equinor/radix-operator/easyauth/internal/defaulter"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/uncachedclient"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
)

const (
	// ClientSecretName is the name of the Secret in the app namespace that holds the OAuth2 client secret of the application
	ClientSecretName = "easyauth-client-secret"
	// ClientSecretKey is the key in ClientSecretName that holds the OAuth2 client secret
	ClientSecretKey = defaults.OAuthClientSecretKeyName

	DefaultResyncInterval = time.Hour
)

// RadixApplicationReconciler reconciles a RadixApplication object
type RadixApplicationReconciler struct {
	client.Client
	// APIReader reads objects that are not in the manager cache, like the client secret in the app namespace
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Annotation that enables EasyAuth on a RadixApplication
	Annotation string
	// OIDC issuer URL set on the OAuth2 configuration of public components
	OAuth2IssuerURL string
	// How often each RadixApplication is reconciled to correct drift. Defaults to DefaultResyncInterval
	ResyncInterval time.Duration
}

// +kubebuilder:rbac:groups=radix.equinor.com,resources=radixapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=radix.equinor.com,resources=radixdeployments,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile ensures that RadixApplications with the EasyAuth annotation have an OAuth2 client secret,
// that the secret is synced to the OAuth2 proxy of each public component, and that all RadixDeployments
// of the application have the EasyAuth OAuth2 configuration on their public components.
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
	if _, ok := ra.GetAnnotations()[r.Annotation]; !ok {
		return ctrl.Result{}, nil
	}

	clientSecret, err := r.ensureClientSecret(ctx, ra)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure client secret: %w", err)
	}

	if err := r.syncComponentSecrets(ctx, ra, clientSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to sync client secret to components: %w", err)
	}

	if err := r.syncRadixDeployments(ctx, ra); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to sync OAuth2 config to RadixDeployments: %w", err)
	}

	logger.Debug().Msg("Reconciled RadixApplication")
	return ctrl.Result{RequeueAfter: r.resyncInterval()}, nil
}

// ensureClientSecret returns the client secret of the application, and generates it if it doesn't exist
func (r *RadixApplicationReconciler) ensureClientSecret(ctx context.Context, ra *radixv1.RadixApplication) ([]byte, error) {
	secret := &corev1.Secret{Name: ClientSecretName, Namespace: ra.Namespace}
	// The client secret is not in the manager cache, so CreateOrUpdate must read it from the API server
	op, err := controllerutil.CreateOrUpdate(ctx, uncachedclient.New(r.Client, r.APIReader), secret, func() error {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[kube.RadixAppLabel] = ra.Name
		if secret.Type == "" {
			secret.Type = corev1.SecretTypeOpaque
		}
		if len(secret.Data[ClientSecretKey]) == 0 {
			if secret.Data == nil {
				secret.Data = map[string][]byte{}
			}
			secret.Data[ClientSecretKey] = generateClientSecret()
		}
		// Deleted together with the RadixApplication
		return ctrl.SetControllerReference(ra, secret, r.Scheme)
	})
	if err != nil {
		return nil, err
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("secret", ClientSecretName).Str("operation", string(op)).Msg("Reconciled client secret")
	}
	return secret.Data[ClientSecretKey], nil
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

func (r *RadixApplicationReconciler) syncComponentSecret(ctx context.Context, namespace, componentName string, clientSecret []byte) error {
	name := utils.GetAuxiliaryComponentSecretName(componentName, radixv1.OAuthProxyAuxiliaryComponentSuffix)
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if bytes.Equal(secret.Data[ClientSecretKey], clientSecret) {
		return nil
	}

	original := secret.DeepCopy()
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[ClientSecretKey] = clientSecret
	if err := r.Patch(ctx, secret, client.MergeFrom(original)); err != nil {
		return err
	}
	log.Ctx(ctx).Info().Str("namespace", namespace).Str("secret", name).Msg("Synced client secret")
	return nil
}

// syncRadixDeployments sets the EasyAuth OAuth2 config on public components of all RadixDeployments in the
// environments of the application. This covers RadixDeployments created before the annotation was added,
// and keeps inactive RadixDeployments correct in case they are promoted later.
func (r *RadixApplicationReconciler) syncRadixDeployments(ctx context.Context, ra *radixv1.RadixApplication) error {
	desired := defaulter.OAuth2Config(ra.Name, r.OAuth2IssuerURL)

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

func (r *RadixApplicationReconciler) syncRadixDeployment(ctx context.Context, rd *radixv1.RadixDeployment, desired *radixv1.OAuth2) error {
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

func (r *RadixApplicationReconciler) resyncInterval() time.Duration {
	if r.ResyncInterval > 0 {
		return r.ResyncInterval
	}
	return DefaultResyncInterval
}

func generateClientSecret() []byte {
	return []byte(rand.Text())
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

func isOAuthProxySecret(obj client.Object) bool {
	return obj.GetLabels()[kube.RadixAuxiliaryComponentTypeLabel] == radixv1.OAuthProxyAuxiliaryComponentType
}

// SetupWithManager sets up the controller with the Manager.
func (r *RadixApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&radixv1.RadixApplication{}).
		Watches(&radixv1.RadixDeployment{},
			handler.EnqueueRequestsFromMapFunc(mapToRadixApplication),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// The OAuth2 proxy Secret is created by radix-operator after the RadixDeployment, so it must be watched to sync the client secret
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(mapToRadixApplication),
			builder.WithPredicates(predicate.NewPredicateFuncs(isOAuthProxySecret)),
		).
		Named("radixapplication").
		Complete(r)
}
