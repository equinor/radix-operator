package environment

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/networkpolicy"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
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
// reconceliated with current state.
func (env *Environment) OnSync(time metav1.Time) error {

	// create a globally unique namespace name
	namespaceName := utils.GetEnvironmentNamespace(env.config.Spec.AppName, env.config.Spec.EnvName)

	err := env.ApplyNamespace(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to apply namespace %s: %v", namespaceName, err)
	}

	err = env.ApplyAdGroupRoleBinding(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to apply RBAC on namespace %s: %v", namespaceName, err)
	}

	err = env.applyRadixTektonEnvRoleBinding(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to apply RBAC for radix-tekton-env on namespace %s: %v", namespaceName, err)
	}

	err = env.ApplyRadixPipelineRunnerRoleBinding(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to apply RBAC for radix-pipeline-runner on namespace %s: %v", namespaceName, err)
	}

	err = env.ApplyLimitRange(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to apply limit range on namespace %s: %v", namespaceName, err)
	}

	err = env.networkPolicy.UpdateEnvEgressRules(env.config.Spec.Egress.Rules, env.config.Spec.Egress.AllowRadix, env.config.Spec.EnvName)
	if err != nil {
		errmsg := fmt.Sprintf("failed to add egress rules in %s, environment %s: ", env.config.Spec.AppName, env.config.Spec.EnvName)
		return fmt.Errorf("%s%v", errmsg, err)
	}

	isOrphaned := !existsInAppConfig(env.appConfig, env.config.Spec.EnvName)

	err = env.updateRadixEnvironmentStatus(env.config, func(currStatus *v1.RadixEnvironmentStatus) {
		currStatus.Orphaned = isOrphaned
		// time is parameterized for testability
		currStatus.Reconciled = time
	})
	if err != nil {
		return fmt.Errorf("failed to update status on environment %s: %v", env.config.Spec.EnvName, err)
	}
	env.logger.Debugf("Environment %s reconciled", namespaceName)
	return nil
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
		"sync":                "radix-wildcard-tls-cert",
		"radix-wildcard-sync": "radix-wildcard-tls-cert",
		imagehubKey:           env.config.Spec.AppName,
		kube.RadixAppLabel:    env.config.Spec.AppName,
		kube.RadixEnvLabel:    env.config.Spec.EnvName,
	}
	nsLabels = labels.Merge(nsLabels, labels.Set(kube.NewPodSecurityStandardFromEnv().Labels()))
	return env.kubeutil.ApplyNamespace(name, nsLabels, env.AsOwnerReference())
}

// ApplyAdGroupRoleBinding grants access to environment namespace
func (env *Environment) ApplyAdGroupRoleBinding(namespace string) error {

	adGroups, err := application.GetAdGroups(env.regConfig)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	if env.regConfig.Spec.MachineUser {
		subjects = append(subjects, rbac.Subject{
			Kind:      k8s.KindServiceAccount,
			Name:      defaults.GetMachineUserRoleName(env.config.Spec.AppName),
			Namespace: utils.GetAppNamespace(env.regConfig.Name),
		})
	}

	roleBinding := kube.GetRolebindingToClusterRoleForSubjects(env.config.Spec.AppName, defaults.AppAdminEnvironmentRoleName, subjects)
	roleBinding.SetOwnerReferences(env.AsOwnerReference())
	err = env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
	if err != nil {
		return err
	}
	return nil
}

func (env *Environment) applyRadixTektonEnvRoleBinding(namespace string) error {
	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.RadixTektonEnvRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: env.config.Spec.AppName,
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.RadixTektonEnvRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      k8s.KindServiceAccount,
				Name:      defaults.RadixTektonServiceAccountName,
				Namespace: utils.GetAppNamespace(env.config.Spec.AppName),
			},
		},
	}
	roleBinding.SetOwnerReferences(env.AsOwnerReference())
	return env.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func (env *Environment) ApplyRadixPipelineRunnerRoleBinding(namespace string) error {
	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.PipelineEnvRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: env.config.Spec.AppName,
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.PipelineEnvRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      k8s.KindServiceAccount,
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

	defaultCPULimit := defaults.GetDefaultCPULimit()
	defaultMemoryLimit := defaults.GetDefaultMemoryLimit()
	defaultCPURequest := defaults.GetDefaultCPURequest()
	defaultMemoryRequest := defaults.GetDefaultMemoryRequest()

	// if not all limits are defined, then don't put any limits on namespace
	if defaultCPULimit == nil ||
		defaultMemoryLimit == nil ||
		defaultCPURequest == nil ||
		defaultMemoryRequest == nil {
		env.logger.Warningf("Not all limits are defined for the Operator, so no limitrange will be put on namespace %s", namespace)
		return nil
	}

	limitRange := env.kubeutil.BuildLimitRange(namespace,
		limitRangeName, env.config.Spec.AppName,
		*defaultCPULimit,
		*defaultMemoryLimit,
		*defaultCPURequest,
		*defaultMemoryRequest)
	limitRange.SetOwnerReferences(env.AsOwnerReference())

	return env.kubeutil.ApplyLimitRange(namespace, limitRange)
}

// AsOwnerReference creates an OwnerReference to this environment object
func (env *Environment) AsOwnerReference() []metav1.OwnerReference {
	trueVar := true
	return []metav1.OwnerReference{
		{
			APIVersion: "radix.equinor.com/v1",
			Kind:       "RadixEnvironment",
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
