package environment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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

	if env.config.ObjectMeta.DeletionTimestamp != nil {
		return env.handleDeletedRadixEnvironment(ctx)
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

func (env *Environment) handleDeletedRadixEnvironment(ctx context.Context) error {
	re := env.config
	env.logger.Debug().Msgf("Handle deleted RadixEnvironment %s in the application %s", re.Name, re.Spec.AppName)
	finalizerIndex := slice.FindIndex(re.ObjectMeta.Finalizers, func(val string) bool {
		return val == kube.RadixEnvironmentFinalizer
	})
	if finalizerIndex < 0 {
		env.logger.Info().Msgf("Missing finalizer %s in the Radix environment %s in the application %s. Exist finalizers: %d. Skip dependency handling",
			kube.RadixEnvironmentFinalizer, re.Name, re.Spec.AppName, len(re.ObjectMeta.Finalizers))
		return nil
	}
	if err := env.handleDeletedRadixEnvironmentDependencies(ctx); err != nil {
		return err
	}
	updatingRE := re.DeepCopy()
	updatingRE.ObjectMeta.Finalizers = append(re.ObjectMeta.Finalizers[:finalizerIndex], re.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	env.logger.Debug().Msgf("Removed finalizer %s from the Radix environment %s in the application %s. Left finalizers: %d",
		kube.RadixEnvironmentFinalizer, updatingRE.Name, updatingRE.Spec.AppName, len(updatingRE.ObjectMeta.Finalizers))
	updated, err := env.kubeutil.UpdateRadixEnvironment(ctx, updatingRE)
	if err != nil {
		return err
	}
	env.logger.Debug().Msgf("Updated RadixEnvironment %s revision %s", re.Name, updated.GetResourceVersion())
	return nil
}

func (env *Environment) handleDeletedRadixEnvironmentDependencies(ctx context.Context) error {
	radixDNSAliasList, err := env.kubeutil.GetRadixDNSAliasWithSelector(ctx, radixlabels.Merge(radixlabels.ForApplicationName(env.config.Spec.AppName), radixlabels.ForEnvironmentName(env.config.Spec.EnvName)).String())
	if err != nil {
		return err
	}
	env.logger.Debug().Msgf("delete %d RadixDNSAlias(es)", len(radixDNSAliasList.Items))
	dnsAliases := slice.Reduce(radixDNSAliasList.Items, []*v1.RadixDNSAlias{}, func(acc []*v1.RadixDNSAlias, radixDNSAlias v1.RadixDNSAlias) []*v1.RadixDNSAlias {
		return append(acc, &radixDNSAlias)
	})
	return env.kubeutil.DeleteRadixDNSAliases(ctx, dnsAliases...)
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
