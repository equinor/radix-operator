package environment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// Environment is the aggregate-root for manipulating RadixEnvironments
type Environment struct {
	kubeclient    kubernetes.Interface
	radixclient   radixclient.Interface
	kubeutil      *kube.Kube
	config        *v1.RadixEnvironment
	regConfig     *v1.RadixRegistration
	appConfig     *v1.RadixApplication
	logger        *logrus.Entry
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
	logger *logrus.Entry,
	networkPolicy *networkpolicy.NetworkPolicy) (Environment, error) {

	return Environment{
		kubeclient,
		radixclient,
		kubeutil,
		config,
		regConfig,
		appConfig,
		logger,
		networkPolicy}, nil
}

// OnSync is called by the handler when changes are applied and must be
// reconciled with current state.
func (env *Environment) OnSync(time metav1.Time) error {
	re := env.config

	if handled, err := env.handleDeletedRadixEnvironment(re); handled || err != nil {
		return err
	}

	if env.regConfig == nil {
		return nil // RadixRegistration does not exist, possible it was deleted
	}

	// create a globally unique namespace name
	namespaceName := utils.GetEnvironmentNamespace(re.Spec.AppName, re.Spec.EnvName)

	if err := env.ApplyNamespace(namespaceName); err != nil {
		return fmt.Errorf("failed to apply namespace %s: %v", namespaceName, err)
	}
	if err := env.ApplyAdGroupRoleBinding(namespaceName); err != nil {
		return fmt.Errorf("failed to apply RBAC on namespace %s: %v", namespaceName, err)
	}
	if err := env.applyRadixTektonEnvRoleBinding(namespaceName); err != nil {
		return fmt.Errorf("failed to apply RBAC for radix-tekton-env on namespace %s: %v", namespaceName, err)
	}
	if err := env.ApplyRadixPipelineRunnerRoleBinding(namespaceName); err != nil {
		return fmt.Errorf("failed to apply RBAC for radix-pipeline-runner on namespace %s: %v", namespaceName, err)
	}
	if err := env.ApplyLimitRange(namespaceName); err != nil {
		return fmt.Errorf("failed to apply limit range on namespace %s: %v", namespaceName, err)
	}
	if err := env.networkPolicy.UpdateEnvEgressRules(re.Spec.Egress.Rules, re.Spec.Egress.AllowRadix, re.Spec.EnvName); err != nil {
		return fmt.Errorf("failed to add egress rules in %s, environment %s: %v", re.Spec.AppName, re.Spec.EnvName, err)
	}

	isOrphaned := !existsInAppConfig(env.appConfig, re.Spec.EnvName)

	err := env.updateRadixEnvironmentStatus(re, func(currStatus *v1.RadixEnvironmentStatus) {
		currStatus.Orphaned = isOrphaned
		// time is parameterized for testability
		currStatus.Reconciled = time
	})
	if err != nil {
		return fmt.Errorf("failed to update status on environment %s: %v", re.Spec.EnvName, err)
	}
	env.logger.Debugf("Environment %s reconciled", namespaceName)
	return nil
}

func (env *Environment) handleDeletedRadixEnvironment(re *v1.RadixEnvironment) (bool, error) {
	if re.ObjectMeta.DeletionTimestamp == nil {
		return false, nil
	}
	logrus.Debugf("handle deleted RadixEnvironment %s in the application %s", re.Name, re.Spec.AppName)
	finalizerIndex := slice.FindIndex(re.ObjectMeta.Finalizers, func(val string) bool {
		return val == kube.RadixEnvironmentFinalizer
	})
	if finalizerIndex < 0 {
		logrus.Infof("missing finalizer %s in the Radix environment %s in the application %s. Exist finalizers: %d. Skip dependency handling",
			kube.RadixEnvironmentFinalizer, re.Name, re.Spec.AppName, len(re.ObjectMeta.Finalizers))
		return false, nil
	}
	err := env.handleDeletedRadixEnvironmentDependencies(re)
	if err != nil {
		return true, err
	}
	updatingRE := re.DeepCopy()
	updatingRE.ObjectMeta.Finalizers = append(re.ObjectMeta.Finalizers[:finalizerIndex], re.ObjectMeta.Finalizers[finalizerIndex+1:]...)
	logrus.Debugf("removed finalizer %s from the Radix environment %s in the application %s. Left finalizers: %d",
		kube.RadixEnvironmentFinalizer, updatingRE.Name, updatingRE.Spec.AppName, len(updatingRE.ObjectMeta.Finalizers))
	if err = env.kubeutil.UpdateRadixEnvironment(updatingRE); err != nil {
		return false, err
	}
	return true, nil
}

func (env *Environment) handleDeletedRadixEnvironmentDependencies(re *v1.RadixEnvironment) error {
	radixDNSAliasList, err := env.kubeutil.GetRadixDNSAliasWithSelector(radixlabels.Merge(radixlabels.ForApplicationName(re.Spec.AppName), radixlabels.ForEnvironmentName(re.Spec.EnvName)).String())
	if err != nil {
		return err
	}
	logrus.Debugf("delete %d RadixDNSAlias(es)", len(radixDNSAliasList.Items))
	return env.kubeutil.DeleteRadixDNSAliases(slice.Reduce(radixDNSAliasList.Items, []*v1.RadixDNSAlias{}, func(acc []*v1.RadixDNSAlias, radixDNSAlias v1.RadixDNSAlias) []*v1.RadixDNSAlias {
		return append(acc, &radixDNSAlias)
	})...)
}

func (env *Environment) updateRadixEnvironmentStatus(rEnv *v1.RadixEnvironment, changeStatusFunc func(currStatus *v1.RadixEnvironmentStatus)) error {
	radixEnvironmentInterface := env.radixclient.RadixV1().RadixEnvironments()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentEnv, err := radixEnvironmentInterface.Get(context.TODO(), rEnv.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changeStatusFunc(&currentEnv.Status)
		_, err = radixEnvironmentInterface.UpdateStatus(context.TODO(), currentEnv, metav1.UpdateOptions{})
		if err == nil && env.config.GetName() == rEnv.GetName() {
			currentEnv, err = radixEnvironmentInterface.Get(context.TODO(), rEnv.GetName(), metav1.GetOptions{})
			if err == nil {
				env.config = currentEnv
			}
		}
		return err
	})
}

// ApplyNamespace sets up namespace metadata and applies configuration to kubernetes
func (env *Environment) ApplyNamespace(name string) error {

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
	return env.kubeutil.ApplyNamespace(name, nsLabels, env.AsOwnerReference())
}

// ApplyAdGroupRoleBinding grants access to environment namespace
func (env *Environment) ApplyAdGroupRoleBinding(namespace string) error {
	adGroups, err := utils.GetAdGroups(env.regConfig)
	if err != nil {
		return err
	}

	adminSubjects := kube.GetRoleBindingGroups(adGroups)
	adminRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(env.config.Spec.AppName, defaults.AppAdminEnvironmentRoleName, adminSubjects)
	adminRoleBinding.SetOwnerReferences(env.AsOwnerReference())

	readerAdGroups := env.regConfig.Spec.ReaderAdGroups
	readerSubjects := kube.GetRoleBindingGroups(readerAdGroups)
	readerRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(env.config.Spec.AppName, defaults.AppReaderEnvironmentsRoleName, readerSubjects)
	readerRoleBinding.SetOwnerReferences(env.AsOwnerReference())

	for _, roleBinding := range []*rbacv1.RoleBinding{adminRoleBinding, readerRoleBinding} {
		err = env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
		if err != nil {
			return err
		}
	}

	return nil
}

func (env *Environment) applyRadixTektonEnvRoleBinding(namespace string) error {
	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.RadixTektonEnvRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: env.config.Spec.AppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.RadixTektonEnvRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      defaults.RadixTektonServiceAccountName,
				Namespace: utils.GetAppNamespace(env.config.Spec.AppName),
			},
		},
	}
	roleBinding.SetOwnerReferences(env.AsOwnerReference())
	return env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func (env *Environment) ApplyRadixPipelineRunnerRoleBinding(namespace string) error {
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
	return env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

const limitRangeName = "mem-cpu-limit-range-env"

// ApplyLimitRange sets resource usage limits to provided namespace
func (env *Environment) ApplyLimitRange(namespace string) error {

	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	defaultCPURequest := defaults.GetDefaultCPURequest()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequest()

	// if not all limits are defined, then don't put any limits on namespace
	if defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		env.logger.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := env.kubeutil.BuildLimitRange(namespace, limitRangeName, env.config.Spec.AppName, defaultMemoryLimit, defaultCPURequest, defaultMemoryRequest)
	limitRange.SetOwnerReferences(env.AsOwnerReference())

	return env.kubeutil.ApplyLimitRange(namespace, limitRange)
}

// AsOwnerReference creates an OwnerReference to this environment object
func (env *Environment) AsOwnerReference() []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: radix.APIVersion,
			Kind:       radix.KindRadixEnvironment,
			Name:       env.config.Name,
			UID:        env.config.UID,
			Controller: &trueVar,
		},
	}
}

func (env *Environment) GetConfig() *v1.RadixEnvironment {
	return env.config
}

func existsInAppConfig(app *v1.RadixApplication, envName string) bool {
	if app == nil {
		return false
	}
	for _, appEnv := range app.Spec.Environments {
		if appEnv.Name == envName {
			return true
		}
	}
	return false
}
