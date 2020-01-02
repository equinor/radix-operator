package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ApplyRole Creates or updates role
func (k *Kube) ApplyRole(namespace string, role *auth.Role) error {
	logger.Debugf("Apply role %s", role.Name)
	oldRole, err := k.GetRole(namespace, role.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdRole, err := k.kubeClient.RbacV1().Roles(namespace).Create(role)
		if err != nil {
			return fmt.Errorf("Failed to create Role object: %v", err)
		}

		log.Debugf("Created Role: %s in namespace %s", createdRole.Name, namespace)
		return nil
	}

	log.Debugf("Role object %s already exists in namespace %s, updating the object now", role.GetName(), namespace)

	newRole := oldRole.DeepCopy()
	newRole.ObjectMeta.OwnerReferences = role.ObjectMeta.OwnerReferences
	newRole.ObjectMeta.Labels = role.Labels
	newRole.Rules = role.Rules

	oldRoleJSON, err := json.Marshal(oldRole)
	if err != nil {
		return fmt.Errorf("Failed to marshal old role object: %v", err)
	}

	newRoleJSON, err := json.Marshal(newRole)
	if err != nil {
		return fmt.Errorf("Failed to marshal new role object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldRoleJSON, newRoleJSON, v1beta1.Role{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch role objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedRole, err := k.kubeClient.RbacV1().Roles(namespace).Patch(role.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch role object: %v", err)
		}
		log.Debugf("Patched role: %s in namespace %s", patchedRole.Name, namespace)
	} else {
		log.Debugf("No need to patch role: %s ", role.GetName())
	}

	return nil
}

// ApplyClusterRole Creates or updates cluster-role
func (k *Kube) ApplyClusterRole(clusterrole *auth.ClusterRole) error {
	logger.Debugf("Apply clusterrole %s", clusterrole.Name)
	oldClusterRole, err := k.GetClusterRole(clusterrole.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdClusterRole, err := k.kubeClient.RbacV1().ClusterRoles().Create(clusterrole)
		if err != nil {
			return fmt.Errorf("Failed to create cluster role object: %v", err)
		}

		log.Debugf("Created cluster role: %s", createdClusterRole.Name)
		return nil
	}

	log.Debugf("Cluster role object %s already exists, updating the object now", clusterrole.GetName())

	newClusterRole := oldClusterRole.DeepCopy()
	newClusterRole.ObjectMeta.OwnerReferences = clusterrole.ObjectMeta.OwnerReferences
	newClusterRole.ObjectMeta.Labels = clusterrole.Labels
	newClusterRole.Rules = clusterrole.Rules

	oldClusterRoleJSON, err := json.Marshal(oldClusterRole)
	if err != nil {
		return fmt.Errorf("Failed to marshal old cluster role object: %v", err)
	}

	newClusterRoleJSON, err := json.Marshal(newClusterRole)
	if err != nil {
		return fmt.Errorf("Failed to marshal new cluster role object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClusterRoleJSON, newClusterRoleJSON, v1beta1.ClusterRole{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch cluster role objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedClusterRole, err := k.kubeClient.RbacV1().ClusterRoles().Patch(clusterrole.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch clusterrole object: %v", err)
		}
		log.Debugf("Patched clusterrole: %s", patchedClusterRole.Name)
	} else {
		log.Debugf("No need to patch clusterrole: %s ", clusterrole.GetName())
	}

	return nil
}

// CreateManageSecretRole creates a role that can manage a secret with predifined set of verbs
func CreateManageSecretRole(appName, roleName string, secretNames []string, customLabels *map[string]string) *auth.Role {
	role := &auth.Role{
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
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: secretNames,
				Verbs:         []string{"get", "list", "watch", "update", "patch", "delete"},
			},
		},
	}
	if customLabels != nil {
		for key, value := range *customLabels {
			role.ObjectMeta.Labels[key] = value
		}
	}

	return role
}

// ListRoles List roles
func (k *Kube) ListRoles(namespace string) ([]*auth.Role, error) {
	return k.ListRolesWithSelector(namespace, nil)
}

// ListRolesWithSelector List roles
func (k *Kube) ListRolesWithSelector(namespace string, labelSelectorString *string) ([]*auth.Role, error) {
	var roles []*auth.Role
	var err error

	if k.RoleLister != nil {
		var selector labels.Selector
		if labelSelectorString != nil {
			labelSelector, err := labelHelpers.ParseToLabelSelector(*labelSelectorString)
			if err != nil {
				return nil, err
			}

			selector, err = labelHelpers.LabelSelectorAsSelector(labelSelector)
			if err != nil {
				return nil, err
			}

		} else {
			selector = labels.NewSelector()
		}

		roles, err = k.RoleLister.Roles(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		list, err := k.kubeClient.RbacV1().Roles(namespace).List(metav1.ListOptions{
			LabelSelector: *labelSelectorString,
		})
		if err != nil {
			return nil, err
		}

		roles = slice.PointersOf(list.Items).([]*auth.Role)
	}

	return roles, nil
}

// GetRole Gets role
func (k *Kube) GetRole(namespace, name string) (*auth.Role, error) {
	var role *auth.Role
	var err error

	if k.RoleLister != nil {
		role, err = k.RoleLister.Roles(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		role, err = k.kubeClient.RbacV1().Roles(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return role, nil
}

// ListClusterRoles List cluster roles
func (k *Kube) ListClusterRoles(namespace string) ([]*auth.ClusterRole, error) {
	var clusterRoles []*auth.ClusterRole
	var err error

	if k.ClusterRoleLister != nil {
		clusterRoles, err = k.ClusterRoleLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := k.kubeClient.RbacV1().ClusterRoles().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		clusterRoles = slice.PointersOf(list.Items).([]*auth.ClusterRole)
	}

	return clusterRoles, nil
}

// GetClusterRole Gets cluster role
func (k *Kube) GetClusterRole(name string) (*auth.ClusterRole, error) {
	var clusterRole *auth.ClusterRole
	var err error

	if k.ClusterRoleLister != nil {
		clusterRole, err = k.ClusterRoleLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		clusterRole, err = k.kubeClient.RbacV1().ClusterRoles().Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return clusterRole, nil
}
