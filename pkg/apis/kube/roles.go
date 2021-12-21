package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyRole Creates or updates role
func (kubeutil *Kube) ApplyRole(namespace string, role *rbacv1.Role) error {
	logger.Debugf("Apply role %s", role.Name)
	oldRole, err := kubeutil.GetRole(namespace, role.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdRole, err := kubeutil.kubeClient.RbacV1().Roles(namespace).Create(context.TODO(), role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Role object: %v", err)
		}

		log.Debugf("Created Role: %s in namespace %s", createdRole.Name, namespace)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get role object: %v", err)
	}

	log.Debugf("Role object %s already exists in namespace %s, updating the object now", role.GetName(), namespace)

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
		patchedRole, err := kubeutil.kubeClient.RbacV1().Roles(namespace).Patch(context.TODO(), role.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch role object: %v", err)
		}
		log.Debugf("Patched role: %s in namespace %s", patchedRole.Name, namespace)
	} else {
		log.Debugf("No need to patch role: %s ", role.GetName())
	}

	return nil
}

// ApplyClusterRole Creates or updates cluster-role
func (kubeutil *Kube) ApplyClusterRole(clusterrole *rbacv1.ClusterRole) error {
	logger.Debugf("Apply clusterrole %s", clusterrole.Name)
	oldClusterRole, err := kubeutil.GetClusterRole(clusterrole.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdClusterRole, err := kubeutil.kubeClient.RbacV1().ClusterRoles().Create(context.TODO(), clusterrole, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role object: %v", err)
		}

		log.Debugf("Created cluster role: %s", createdClusterRole.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get cluster role object: %v", err)
	}

	log.Debugf("Cluster role object %s already exists, updating the object now", clusterrole.GetName())

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
		patchedClusterRole, err := kubeutil.kubeClient.RbacV1().ClusterRoles().Patch(context.TODO(), clusterrole.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch clusterrole object: %v", err)
		}
		log.Debugf("Patched clusterrole: %s", patchedClusterRole.Name)
	} else {
		log.Debugf("No need to patch clusterrole: %s ", clusterrole.GetName())
	}

	return nil
}

// CreateManageSecretRole creates a role that can manage a secret with predefined set of verbs
func CreateManageSecretRole(appName, roleName string, secretNames []string, customLabels map[string]string) *rbacv1.Role {
	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixReg":    appName, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: appName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: secretNames,
				Verbs:         []string{"get", "list", "watch", "update", "patch", "delete"},
			},
		},
	}

	for key, value := range customLabels {
		role.ObjectMeta.Labels[key] = value
	}

	return role
}

// ListRoles List roles
func (kubeutil *Kube) ListRoles(namespace string) ([]*rbacv1.Role, error) {
	return kubeutil.ListRolesWithSelector(namespace, "")
}

// ListRolesWithSelector List roles
func (kubeutil *Kube) ListRolesWithSelector(namespace string, labelSelectorString string) ([]*rbacv1.Role, error) {
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
			context.TODO(),
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
func (kubeutil *Kube) GetRole(namespace, name string) (*rbacv1.Role, error) {
	var role *rbacv1.Role
	var err error

	if kubeutil.RoleLister != nil {
		role, err = kubeutil.RoleLister.Roles(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		role, err = kubeutil.kubeClient.RbacV1().Roles(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return role, nil
}

// ListClusterRoles List cluster roles
func (kubeutil *Kube) ListClusterRoles(namespace string) ([]*rbacv1.ClusterRole, error) {
	var clusterRoles []*rbacv1.ClusterRole
	var err error

	if kubeutil.ClusterRoleLister != nil {
		clusterRoles, err = kubeutil.ClusterRoleLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := kubeutil.kubeClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		clusterRoles = slice.PointersOf(list.Items).([]*rbacv1.ClusterRole)
	}

	return clusterRoles, nil
}

// DeleteRole Deletes a role in a namespace
func (kubeutil *Kube) DeleteRole(namespace, name string) error {
	_, err := kubeutil.GetRole(namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get role object: %v", err)
	}
	err = kubeutil.kubeClient.RbacV1().Roles(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete role object: %v", err)
	}
	return nil
}

// GetClusterRole Gets cluster role
func (kubeutil *Kube) GetClusterRole(name string) (*rbacv1.ClusterRole, error) {
	var clusterRole *rbacv1.ClusterRole
	var err error

	if kubeutil.ClusterRoleLister != nil {
		clusterRole, err = kubeutil.ClusterRoleLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		clusterRole, err = kubeutil.kubeClient.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return clusterRole, nil
}
