package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/rs/zerolog/log"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyRole Creates or updates role
func (kubeutil *Kube) ApplyRole(ctx context.Context, namespace string, role *rbacv1.Role) error {
	log.Debug().Msgf("Apply role %s", role.Name)
	oldRole, err := kubeutil.GetRole(ctx, namespace, role.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdRole, err := kubeutil.kubeClient.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Role object: %v", err)
		}

		log.Debug().Msgf("Created Role: %s in namespace %s", createdRole.Name, namespace)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get role object: %v", err)
	}

	log.Debug().Msgf("Role object %s already exists in namespace %s, updating the object now", role.GetName(), namespace)

	newRole := oldRole.DeepCopy()
	newRole.ObjectMeta.OwnerReferences = role.ObjectMeta.OwnerReferences
	newRole.ObjectMeta.Labels = role.Labels
	newRole.Rules = role.Rules

	oldRoleJSON, err := json.Marshal(oldRole)
	if err != nil {
		return fmt.Errorf("failed to marshal old role object: %v", err)
	}

	newRoleJSON, err := json.Marshal(newRole)
	if err != nil {
		return fmt.Errorf("failed to marshal new role object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldRoleJSON, newRoleJSON, rbacv1.Role{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch role objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedRole, err := kubeutil.kubeClient.RbacV1().Roles(namespace).Patch(ctx, role.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch role object: %v", err)
		}
		log.Debug().Msgf("Patched role: %s in namespace %s", patchedRole.Name, namespace)
	} else {
		log.Debug().Msgf("No need to patch role: %s ", role.GetName())
	}

	return nil
}

// ApplyClusterRole Creates or updates cluster-role
func (kubeutil *Kube) ApplyClusterRole(ctx context.Context, clusterrole *rbacv1.ClusterRole) error {
	log.Debug().Msgf("Apply clusterrole %s", clusterrole.Name)
	oldClusterRole, err := kubeutil.kubeClient.RbacV1().ClusterRoles().Get(ctx, clusterrole.GetName(), metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		createdClusterRole, err := kubeutil.kubeClient.RbacV1().ClusterRoles().Create(ctx, clusterrole, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role object: %v", err)
		}

		log.Debug().Msgf("Created cluster role: %s", createdClusterRole.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get cluster role object: %v", err)
	}

	log.Debug().Msgf("Cluster role object %s already exists, updating the object now", clusterrole.GetName())

	newClusterRole := oldClusterRole.DeepCopy()
	newClusterRole.ObjectMeta.OwnerReferences = clusterrole.ObjectMeta.OwnerReferences
	newClusterRole.ObjectMeta.Labels = clusterrole.Labels
	newClusterRole.Rules = clusterrole.Rules

	oldClusterRoleJSON, err := json.Marshal(oldClusterRole)
	if err != nil {
		return fmt.Errorf("failed to marshal old cluster role object: %v", err)
	}

	newClusterRoleJSON, err := json.Marshal(newClusterRole)
	if err != nil {
		return fmt.Errorf("failed to marshal new cluster role object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClusterRoleJSON, newClusterRoleJSON, rbacv1.ClusterRole{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch cluster role objects: %v", err)
	}

	if !IsEmptyPatch(patchBytes) {
		patchedClusterRole, err := kubeutil.kubeClient.RbacV1().ClusterRoles().Patch(ctx, clusterrole.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch clusterrole object: %v", err)
		}
		log.Debug().Msgf("Patched clusterrole: %s", patchedClusterRole.Name)
	} else {
		log.Debug().Msgf("No need to patch clusterrole: %s ", clusterrole.GetName())
	}

	return nil
}

type RuleBuilder func() rbacv1.PolicyRule

func ManageSecretsRule(secretNames []string) RuleBuilder {
	return func() rbacv1.PolicyRule {
		return rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: secretNames,
			Verbs:         []string{"get", "list", "watch", "update", "patch", "delete"},
		}
	}
}

func ReadSecretsRule(secretNames []string) RuleBuilder {
	return func() rbacv1.PolicyRule {
		return rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: secretNames,
			Verbs:         []string{"get", "list", "watch"},
		}
	}
}

func UpdateDeploymentsRule(deployments []string) RuleBuilder {
	return func() rbacv1.PolicyRule {
		return rbacv1.PolicyRule{
			APIGroups:     []string{"apps"},
			Resources:     []string{"deployments"},
			ResourceNames: deployments,
			Verbs:         []string{"update"},
		}
	}
}

func CreateAppRole(appName, roleName string, customLabels map[string]string, ruleBuilders ...RuleBuilder) *rbacv1.Role {
	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindRole,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				RadixAppLabel: appName,
			},
		},
	}

	for _, rb := range ruleBuilders {
		role.Rules = append(role.Rules, rb())
	}

	for key, value := range customLabels {
		role.ObjectMeta.Labels[key] = value
	}

	return role
}

// CreateManageSecretRole creates a role that can manage a secret with predefined set of verbs
func CreateManageSecretRole(appName, roleName string, secretNames []string, customLabels map[string]string) *rbacv1.Role {
	return CreateAppRole(appName, roleName, customLabels, ManageSecretsRule(secretNames))
}

// CreateReadSecretRole creates a role that can read a secret with predefined set of verbs
func CreateReadSecretRole(appName, roleName string, secretNames []string, customLabels map[string]string) *rbacv1.Role {
	return CreateAppRole(appName, roleName, customLabels, ReadSecretsRule(secretNames))
}

// ListRoles List roles
func (kubeutil *Kube) ListRoles(ctx context.Context, namespace string) ([]*rbacv1.Role, error) {
	return kubeutil.ListRolesWithSelector(ctx, namespace, "")
}

// ListRolesWithSelector List roles
func (kubeutil *Kube) ListRolesWithSelector(ctx context.Context, namespace string, labelSelectorString string) ([]*rbacv1.Role, error) {
	var roles []*rbacv1.Role

	if kubeutil.RoleLister != nil {
		selector, err := labels.Parse(labelSelectorString)
		if err != nil {
			return nil, err
		}

		roles, err = kubeutil.RoleLister.Roles(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.kubeClient.RbacV1().Roles(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: labelSelectorString,
			})
		if err != nil {
			return nil, err
		}

		roles = slice.PointersOf(list.Items).([]*rbacv1.Role)
	}

	return roles, nil
}

// GetRole Gets role
func (kubeutil *Kube) GetRole(ctx context.Context, namespace, name string) (*rbacv1.Role, error) {
	var role *rbacv1.Role
	var err error

	if kubeutil.RoleLister != nil {
		role, err = kubeutil.RoleLister.Roles(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		role, err = kubeutil.kubeClient.RbacV1().Roles(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return role, nil
}

// ListClusterRolesWithSelector List cluster roles
func (kubeutil *Kube) ListClusterRolesWithSelector(ctx context.Context, labelSelectorString string) ([]*rbacv1.ClusterRole, error) {
	var clusterRoles []*rbacv1.ClusterRole
	selector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return nil, err
	}

	if kubeutil.ClusterRoleLister != nil {
		clusterRoles, err = kubeutil.ClusterRoleLister.List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.kubeClient.RbacV1().ClusterRoles().List(ctx,
			metav1.ListOptions{
				LabelSelector: labelSelectorString,
			})
		if err != nil {
			return nil, err
		}

		clusterRoles = slice.PointersOf(list.Items).([]*rbacv1.ClusterRole)
	}

	return clusterRoles, nil
}

// DeleteRole Deletes a role in a namespace
func (kubeutil *Kube) DeleteRole(ctx context.Context, namespace, name string) error {
	_, err := kubeutil.GetRole(ctx, namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get role object: %v", err)
	}
	err = kubeutil.kubeClient.RbacV1().Roles(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete role object: %v", err)
	}
	return nil
}

// DeleteClusterRole Deletes a role in a namespace
func (kubeutil *Kube) DeleteClusterRole(ctx context.Context, name string) error {
	err := kubeutil.kubeClient.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete cluster role object: %v", err)
	}
	return nil
}

// GetClusterRole Gets cluster role
func (kubeutil *Kube) GetClusterRole(ctx context.Context, name string) (*rbacv1.ClusterRole, error) {
	var clusterRole *rbacv1.ClusterRole
	var err error

	if kubeutil.ClusterRoleLister != nil {
		clusterRole, err = kubeutil.ClusterRoleLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		clusterRole, err = kubeutil.kubeClient.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return clusterRole, nil
}
