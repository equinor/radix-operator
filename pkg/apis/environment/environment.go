package environment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	finalizer = "radix.equinor.com/environment-finalizer"
)

// Environment is the aggregate-root for manipulating RadixEnvironments
type Environment struct {
	kubeclient    kubernetes.Interface
	radixclient   radixclient.Interface
	kubeutil      *kube.Kube
	config        *v1.RadixEnvironment
	regConfig     *v1.RadixRegistration
	appConfig     *v1.RadixApplication
	logger        zerolog.Logger
	networkPolicy *networkpolicy.NetworkPolicy
}

// NewEnvironment is the constructor for Environment
func NewEnvironment(
	kubeclient kubernetes.Interface,
	kubeutil *kube.Kube,
	radixclient radixclient.Interface,
	config *v1.RadixEnvironment,
	regConfig *v1.RadixRegistration,
	appConfig *v1.RadixApplication,
	networkPolicy *networkpolicy.NetworkPolicy) Environment {

	return Environment{
		kubeclient:    kubeclient,
		radixclient:   radixclient,
		kubeutil:      kubeutil,
		config:        config,
		regConfig:     regConfig,
		appConfig:     appConfig,
		networkPolicy: networkPolicy,
		logger:        log.Logger.With().Str("resource_kind", v1.KindRadixEnvironment).Str("resource_name", cache.MetaObjectToName(&config.ObjectMeta).String()).Logger(),
	}
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (env *Environment) OnSync(ctx context.Context) error {

	if err := env.removeFinalizer(ctx); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	if env.regConfig == nil {
		return nil // RadixRegistration does not exist, possible it was deleted
	}

	return env.syncStatus(ctx, env.reconcile(ctx))
}

func (env *Environment) reconcile(ctx context.Context) error {
	if err := env.applyNamespace(ctx); err != nil {
		return fmt.Errorf("failed to reconcile environment namespace: %w", err)
	}
	if err := env.applyAdGroupRoleBinding(ctx); err != nil {
		return fmt.Errorf("failed to apply user RBAC: %w", err)
	}
	if err := env.applyRadixPipelineRunnerRoleBinding(ctx); err != nil {
		return fmt.Errorf("failed to apply pipeline RBAC: %w", err)
	}
	if err := env.applyLimitRange(ctx); err != nil {
		return fmt.Errorf("failed to apply limit range: %w", err)
	}
	if err := env.networkPolicy.UpdateEnvEgressRules(ctx, env.config.Spec.Egress.Rules, env.config.Spec.Egress.AllowRadix, env.config.Spec.EnvName); err != nil {
		return fmt.Errorf("failed to add egress rules: %w", err)
	}

	return nil
}

// removeFinalizer removes the finalizer that was unneccessarily set to delete ingresses and cluster roles + binding
// ownerreference to the RadixDNSAlias will handle deletion automatically
func (env *Environment) removeFinalizer(ctx context.Context) error {
	log.Ctx(ctx).Info().Msg("Process finalizer")
	if !controllerutil.ContainsFinalizer(env.config, finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(env.config, finalizer)
	updated, err := env.radixclient.RadixV1().RadixEnvironments().Update(ctx, env.config, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	env.config = updated
	return nil
}

// applyNamespace sets up namespace metadata and applies configuration to kubernetes
func (env *Environment) applyNamespace(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)
	// get key to use for namespace annotation to pick up private image hubs
	imagehubKey := fmt.Sprintf("%s-sync", defaults.PrivateImageHubSecretName)
	nsLabels := labels.Set{
		"sync":                         "cluster-wildcard-tls-cert",
		"cluster-wildcard-sync":        "cluster-wildcard-tls-cert",        // redundant, can be removed
		"app-wildcard-sync":            "app-wildcard-tls-cert",            // redundant, can be removed
		"active-cluster-wildcard-sync": "active-cluster-wildcard-tls-cert", // redundant, can be removed
		"radix-wildcard-sync":          "radix-wildcard-tls-cert",
		imagehubKey:                    env.config.Spec.AppName,
		kube.RadixAppLabel:             env.config.Spec.AppName,
		kube.RadixEnvLabel:             env.config.Spec.EnvName,
	}
	nsLabels = labels.Merge(nsLabels, kube.NewEnvNamespacePodSecurityStandardFromEnv().Labels())
	return env.kubeutil.ApplyNamespace(ctx, namespace, nsLabels, env.AsOwnerReference())
}

// applyAdGroupRoleBinding grants access to environment namespace
func (env *Environment) applyAdGroupRoleBinding(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)
	adminSubjects := utils.GetAppAdminRbacSubjects(env.regConfig)
	adminRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(env.config.Spec.AppName, defaults.AppAdminEnvironmentRoleName, adminSubjects)
	adminRoleBinding.SetOwnerReferences(env.AsOwnerReference())

	readerSubjects := utils.GetAppReaderRbacSubjects(env.regConfig)
	readerRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(env.config.Spec.AppName, defaults.AppReaderEnvironmentsRoleName, readerSubjects)
	readerRoleBinding.SetOwnerReferences(env.AsOwnerReference())

	for _, roleBinding := range []*rbacv1.RoleBinding{adminRoleBinding, readerRoleBinding} {
		if err := env.kubeutil.ApplyRoleBinding(ctx, namespace, roleBinding); err != nil {
			return err
		}
	}

	return nil
}

func (env *Environment) applyRadixPipelineRunnerRoleBinding(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)
	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.PipelineEnvRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: env.config.Spec.AppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.PipelineEnvRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      defaults.PipelineServiceAccountName,
				Namespace: utils.GetAppNamespace(env.config.Spec.AppName),
			},
		},
	}
	roleBinding.SetOwnerReferences(env.AsOwnerReference())
	return env.kubeutil.ApplyRoleBinding(ctx, namespace, roleBinding)
}

const limitRangeName = "mem-cpu-limit-range-env"

// applyLimitRange sets resource usage limits to provided namespace
func (env *Environment) applyLimitRange(ctx context.Context) error {
	namespace := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	defaultCPURequest := defaults.GetDefaultCPURequest()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequest()

	// if not all limits are defined, then don't put any limits on namespace
	if defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		env.logger.Warn().Msgf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := env.kubeutil.BuildLimitRange(namespace, limitRangeName, env.config.Spec.AppName, defaultMemoryLimit, defaultCPURequest, defaultMemoryRequest)
	limitRange.SetOwnerReferences(env.AsOwnerReference())

	return env.kubeutil.ApplyLimitRange(ctx, namespace, limitRange)
}

// AsOwnerReference creates an OwnerReference to this environment object
func (env *Environment) AsOwnerReference() []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: v1.SchemeGroupVersion.Identifier(),
			Kind:       v1.KindRadixEnvironment,
			Name:       env.config.Name,
			UID:        env.config.UID,
			Controller: &trueVar,
		},
	}
}

func (env *Environment) GetConfig() *v1.RadixEnvironment {
	return env.config
}
