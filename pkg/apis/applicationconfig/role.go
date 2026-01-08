package applicationconfig

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *ApplicationConfig) grantAccessToDNSAliases(ctx context.Context) error {
	err := app.grantAppAdminAccessToDNSAliases(ctx)
	if err != nil {
		return err
	}

	return app.grantAppReaderAccessToDNSAliases(ctx)
}

func (app *ApplicationConfig) garbageCollectAccessToDNSAliases(ctx context.Context) error {
	if err := app.garbageCollectAppAdminAccessToDNSAliases(ctx); err != nil {
		return err
	}

	return app.garbageCollectAppReaderAccessToDNSAliases(ctx)
}

func (app *ApplicationConfig) garbageCollectAppAdminAccessToDNSAliases(ctx context.Context) error {
	if err := app.deleteDNSAliasesClusterRoleAndBinding(ctx, app.getAppAdminAccessToDNSAliasClusterRoleName()); err != nil {
		return fmt.Errorf("failed to garbage collect DNSAlias app admin access: %w", err)
	}

	return nil
}

func (app *ApplicationConfig) garbageCollectAppReaderAccessToDNSAliases(ctx context.Context) error {
	if err := app.deleteDNSAliasesClusterRoleAndBinding(ctx, app.getAppReaderAccessToDNSAliasClusterRoleName()); err != nil {
		return fmt.Errorf("failed to garbage collect DNSAlias app reader access: %w", err)
	}

	return nil
}

func (app *ApplicationConfig) deleteDNSAliasesClusterRoleAndBinding(ctx context.Context, name string) error {
	if err := app.kubeclient.RbacV1().ClusterRoleBindings().Delete(ctx, name, v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err := app.kubeclient.RbacV1().ClusterRoles().Delete(ctx, name, v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (app *ApplicationConfig) grantAppAdminAccessToDNSAliases(ctx context.Context) error {
	roleName := app.getAppAdminAccessToDNSAliasClusterRoleName()
	verbs := []string{"get", "list", "watch"}
	subjects := utils.GetAppAdminRbacSubjects(app.registration)

	if err := app.createOrUpdateDNSAliasClusterRoleAndBinding(ctx, roleName, verbs, subjects); err != nil {
		return fmt.Errorf("failed to grant app admin access to DNSAlias: %w", err)
	}

	return nil
}

func (app *ApplicationConfig) grantAppReaderAccessToDNSAliases(ctx context.Context) error {
	roleName := app.getAppReaderAccessToDNSAliasClusterRoleName()
	verbs := []string{"get", "list", "watch"}
	subjects := utils.GetAppReaderRbacSubjects(app.registration)

	if err := app.createOrUpdateDNSAliasClusterRoleAndBinding(ctx, roleName, verbs, subjects); err != nil {
		return fmt.Errorf("failed to grant app reader access to DNSAlias: %w", err)
	}

	return nil
}

func (app *ApplicationConfig) createOrUpdateDNSAliasClusterRoleAndBinding(ctx context.Context, name string, verbs []string, subjects []rbacv1.Subject) error {
	role, err := app.createOrUpdateDNSAliasClusterRole(ctx, name, verbs)
	if err != nil {
		return err
	}

	if err := app.createOrUpdateClusterRoleBinding(ctx, role, subjects); err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) getAppAdminAccessToDNSAliasClusterRoleName() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationAdminRadixDNSAliasRoleNamePrefix, app.registration.Name)
}

func (app *ApplicationConfig) getAppReaderAccessToDNSAliasClusterRoleName() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationReaderRadixDNSAliasRoleNamePrefix, app.registration.Name)
}

func (app *ApplicationConfig) createOrUpdateDNSAliasClusterRole(ctx context.Context, name string, verbs []string) (*rbacv1.ClusterRole, error) {
	var current, desired *rbacv1.ClusterRole
	current, err := app.kubeclient.RbacV1().ClusterRoles().Get(ctx, name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get current cluster role: %w", err)
		}
		current = nil
		desired = &rbacv1.ClusterRole{
			ObjectMeta: v1.ObjectMeta{
				Name: name,
			},
		}
	} else {
		desired = current.DeepCopy()
	}

	desired.OwnerReferences = []v1.OwnerReference{}
	if err := controllerutil.SetControllerReference(app.registration, desired, scheme); err != nil {
		return nil, fmt.Errorf("failed to set ownerreference for cluster role: %w", err)
	}
	delete(desired.Labels, kube.RadixAliasLabel) // Delete unneeded label
	desired.Labels = labels.Merge(desired.Labels, labels.ForApplicationName(app.registration.Name))
	desired.Rules = []rbacv1.PolicyRule{
		{
			Verbs:         verbs,
			APIGroups:     []string{radixv1.SchemeGroupVersion.Group},
			Resources:     []string{radixv1.ResourceRadixDNSAliases},
			ResourceNames: slice.Map(app.config.Spec.DNSAlias, func(a radixv1.DNSAlias) string { return a.Alias }),
		},
	}

	if current != nil {
		if !equality.Semantic.DeepEqual(current, desired) {
			desired, err = app.kubeclient.RbacV1().ClusterRoles().Update(ctx, desired, v1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update cluster role: %w", err)
			}
		}
		return desired, nil
	}

	desired, err = app.kubeclient.RbacV1().ClusterRoles().Create(ctx, desired, v1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster role: %w", err)
	}

	return desired, nil

}

func (app *ApplicationConfig) createOrUpdateClusterRoleBinding(ctx context.Context, clusterRole *rbacv1.ClusterRole, subjects []rbacv1.Subject) error {
	var currentBinding, desiredBinding *rbacv1.ClusterRoleBinding
	currentBinding, err := app.kubeclient.RbacV1().ClusterRoleBindings().Get(ctx, clusterRole.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get current cluster role binding: %w", err)
		}
		currentBinding = nil
		desiredBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Name: clusterRole.Name,
			},
		}
	} else {
		desiredBinding = currentBinding.DeepCopy()
	}

	desiredBinding.OwnerReferences = []v1.OwnerReference{}
	if err := controllerutil.SetControllerReference(app.registration, desiredBinding, scheme); err != nil {
		return fmt.Errorf("failed to set ownerreference for DNSAlias admin cluster role: %w", err)
	}
	delete(desiredBinding.Labels, kube.RadixAliasLabel) // Delete unneeded label
	desiredBinding.Labels = labels.Merge(desiredBinding.Labels, labels.ForApplicationName(app.registration.Name))
	desiredBinding.Subjects = subjects
	desiredBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}

	if currentBinding != nil {
		if !equality.Semantic.DeepEqual(currentBinding, desiredBinding) {
			if _, err := app.kubeclient.RbacV1().ClusterRoleBindings().Update(ctx, desiredBinding, v1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update cluster role binding: %w", err)
			}
		}
	} else {
		if _, err := app.kubeclient.RbacV1().ClusterRoleBindings().Create(ctx, desiredBinding, v1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create cluster role binding: %w", err)
		}
	}

	return nil
}

func (app *ApplicationConfig) grantAccessToBuildSecrets(ctx context.Context) error {
	namespace := utils.GetAppNamespace(app.config.Name)
	err := app.grantPipelineAccessToSecret(ctx, namespace, defaults.BuildSecretsName)
	if err != nil {
		return err
	}

	err = app.grantAppReaderAccessToBuildSecrets(ctx, namespace)
	if err != nil {
		return err
	}

	err = app.grantAppAdminAccessToBuildSecrets(ctx, namespace)
	if err != nil {
		return err
	}

	return nil
}

func (app *ApplicationConfig) grantAppReaderAccessToBuildSecrets(ctx context.Context, namespace string) error {
	role := roleAppReaderBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppReaderToBuildSecrets(app.GetRadixRegistration(), role)
	return app.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)

}

func (app *ApplicationConfig) grantAppAdminAccessToBuildSecrets(ctx context.Context, namespace string) error {
	role := roleAppAdminBuildSecrets(app.GetRadixRegistration(), defaults.BuildSecretsName)
	err := app.kubeutil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminToBuildSecrets(app.GetRadixRegistration(), role)
	return app.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (app *ApplicationConfig) grantPipelineAccessToSecret(ctx context.Context, namespace, secretName string) error {
	role := rolePipelineSecret(app.GetRadixRegistration(), secretName)
	err := app.kubeutil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingPipelineToRole(role)
	return app.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (app *ApplicationConfig) garbageCollectAccessToBuildSecretsForRole(ctx context.Context, namespace string, roleName string) error {
	// Delete role
	_, err := app.kubeutil.GetRole(ctx, namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = app.kubeutil.DeleteRole(ctx, namespace, roleName)
		if err != nil {
			return err
		}
	}

	// Delete roleBinding
	_, err = app.kubeutil.GetRoleBinding(ctx, namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = app.kubeutil.DeleteRoleBinding(ctx, namespace, roleName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *ApplicationConfig) garbageCollectAccessToBuildSecrets(ctx context.Context) error {
	appNamespace := utils.GetAppNamespace(app.config.Name)
	for _, roleName := range []string{
		getPipelineRoleNameToSecret(defaults.BuildSecretsName),
		getAppReaderRoleNameToBuildSecrets(defaults.BuildSecretsName),
		getAppAdminRoleNameToBuildSecrets(defaults.BuildSecretsName),
	} {
		err := app.garbageCollectAccessToBuildSecretsForRole(ctx, appNamespace, roleName)
		if err != nil {
			return err
		}
	}
	return nil
}

func roleAppAdminBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *rbacv1.Role {
	return kube.CreateManageSecretRole(registration.Name, getAppAdminRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func roleAppReaderBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *rbacv1.Role {
	return kube.CreateReadSecretRole(registration.Name, getAppReaderRoleNameToBuildSecrets(buildSecretName), []string{buildSecretName}, nil)
}

func rolePipelineSecret(registration *radixv1.RadixRegistration, secretName string) *rbacv1.Role {
	return kube.CreateReadSecretRole(registration.Name, getPipelineRoleNameToSecret(secretName), []string{secretName}, nil)
}

func getAppAdminRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppAdminRoleName, buildSecretName)
}

func getAppReaderRoleNameToBuildSecrets(buildSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppReaderRoleName, buildSecretName)
}

func getPipelineRoleNameToSecret(secretName string) string {
	return fmt.Sprintf("%s-%s", "pipeline", secretName)
}
