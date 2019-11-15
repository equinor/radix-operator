package kube

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// GetRoleBindingGroups Get subjects for list of ad groups
func GetRoleBindingGroups(groups []string) []auth.Subject {
	subjects := []auth.Subject{}
	for _, group := range groups {
		subjects = append(subjects, auth.Subject{
			Kind:     "Group",
			Name:     group,
			APIGroup: "rbac.authorization.k8s.io",
		})
	}
	return subjects
}

// GetRolebindingToRole Get role binding object
func GetRolebindingToRole(appName, name string, groups []string) *auth.RoleBinding {
	return GetRolebindingToRoleWithLabels(name, groups, map[string]string{
		RadixAppLabel: appName,
	})
}

// GetRolebindingToRoleWithLabels Get role binding object
func GetRolebindingToRoleWithLabels(name string, groups []string, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForGroups(name, "Role", groups, labels)
}

// GetRolebindingToClusterRole Get role binding object
func GetRolebindingToClusterRole(appName, name string, groups []string) *auth.RoleBinding {
	return GetRolebindingToClusterRoleWithLabels(name, groups, map[string]string{
		RadixAppLabel: appName,
	})
}

// GetRolebindingToClusterRoleWithLabels Get role binding object
func GetRolebindingToClusterRoleWithLabels(name string, groups []string, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForGroups(name, "ClusterRole", groups, labels)
}

// GetRolebindingToRoleForServiceAccountWithLabels Get role binding object
func GetRolebindingToRoleForServiceAccountWithLabels(name, serviceAccountName, serviceAccountNamespace string, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForServiceAccount(name, "Role", serviceAccountName, serviceAccountNamespace, labels)
}

// GetRolebindingToClusterRoleForServiceAccountWithLabels Get role binding object
func GetRolebindingToClusterRoleForServiceAccountWithLabels(name, serviceAccountName, serviceAccountNamespace string, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForServiceAccount(name, "ClusterRole", serviceAccountName, serviceAccountNamespace, labels)
}

func getRoleBindingForGroups(name, kind string, groups []string, labels map[string]string) *auth.RoleBinding {
	subjects := GetRoleBindingGroups(groups)
	return &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     kind,
			Name:     name,
		},
		Subjects: subjects,
	}
}

func getRoleBindingForServiceAccount(name, kind, serviceAccountName, serviceAccountNamespace string, labels map[string]string) *auth.RoleBinding {
	return &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     kind,
			Name:     name,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: serviceAccountNamespace,
			},
		},
	}
}

// ApplyRoleBinding Creates or updates role-binding
func (k *Kube) ApplyRoleBinding(namespace string, rolebinding *auth.RoleBinding) error {
	logger = logger.WithFields(log.Fields{"roleBinding": rolebinding.ObjectMeta.Name})

	logger.Debugf("Apply rolebinding %s", rolebinding.Name)

	_, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		logger.Debugf("Rolebinding %s already exists, updating the object now", rolebinding.Name)
		oldRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Get(rolebinding.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old role binding object: %v", err)
		}

		newRoleBinding := oldRoleBinding.DeepCopy()
		newRoleBinding.ObjectMeta.OwnerReferences = rolebinding.OwnerReferences
		newRoleBinding.ObjectMeta.Labels = rolebinding.Labels
		newRoleBinding.Subjects = rolebinding.Subjects

		oldRoleBindingJSON, err := json.Marshal(oldRoleBinding)
		if err != nil {
			return fmt.Errorf("Failed to marshal old role binding object: %v", err)
		}

		newRoleBindingJSON, err := json.Marshal(newRoleBinding)
		if err != nil {
			return fmt.Errorf("Failed to marshal new role binding object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldRoleBindingJSON, newRoleBindingJSON, auth.RoleBinding{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch role binding objects: %v", err)
		}

		if !isEmptyPatch(patchBytes) {
			patchedRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Patch(rolebinding.Name, types.StrategicMergePatchType, patchBytes)
			if err != nil {
				return fmt.Errorf("Failed to patch role binding object: %v", err)
			}

			log.Debugf("Patched role binding: %s ", patchedRoleBinding.Name)
		} else {
			log.Debugf("No need to patch role binding: %s ", rolebinding.Name)
		}

		return nil
	}

	if err != nil {
		logger.Errorf("Failed to save roleBinding in [%s]: %v", namespace, err)
		return err
	}

	logger.Debugf("Created roleBinding %s in %s", rolebinding.Name, namespace)
	return nil
}

// ApplyClusterRoleBinding Creates or updates cluster-role-binding
func (k *Kube) ApplyClusterRoleBinding(clusterrolebinding *auth.ClusterRoleBinding) error {
	logger = logger.WithFields(log.Fields{"clusterRoleBinding": clusterrolebinding.ObjectMeta.Name})

	logger.Debugf("Apply clusterrolebinding %s", clusterrolebinding.Name)

	_, err := k.kubeClient.RbacV1().ClusterRoleBindings().Create(clusterrolebinding)
	if errors.IsAlreadyExists(err) {
		logger.Debugf("ClusterRolebinding %s already exists, updating the object now", clusterrolebinding.Name)
		oldClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Get(clusterrolebinding.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get old clusterrole binding object: %v", err)
		}

		newClusterRoleBinding := oldClusterRoleBinding.DeepCopy()
		newClusterRoleBinding.ObjectMeta.OwnerReferences = clusterrolebinding.OwnerReferences
		newClusterRoleBinding.ObjectMeta.Labels = clusterrolebinding.Labels
		newClusterRoleBinding.Subjects = clusterrolebinding.Subjects

		oldClusterRoleBindingJSON, err := json.Marshal(oldClusterRoleBinding)
		if err != nil {
			return fmt.Errorf("Failed to marshal old clusterrole binding object: %v", err)
		}

		newClusterRoleBindingJSON, err := json.Marshal(newClusterRoleBinding)
		if err != nil {
			return fmt.Errorf("Failed to marshal new clusterrole binding object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClusterRoleBindingJSON, newClusterRoleBindingJSON, auth.ClusterRoleBinding{})
		if err != nil {
			return fmt.Errorf("Failed to create two way merge patch clusterrole binding objects: %v", err)
		}

		if !isEmptyPatch(patchBytes) {
			patchedClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Patch(clusterrolebinding.Name, types.StrategicMergePatchType, patchBytes)
			if err != nil {
				return fmt.Errorf("Failed to patch clusterrole binding object: %v", err)
			}

			log.Debugf("Patched clusterrole binding: %s ", patchedClusterRoleBinding.Name)
		} else {
			log.Debugf("No need to patch clusterrole binding: %s ", clusterrolebinding.Name)
		}

		return nil
	}

	if err != nil {
		logger.Errorf("Failed to create clusterRoleBinding: %v", err)
		return err
	}

	logger.Debugf("Created clusterRoleBinding %s", clusterrolebinding.Name)
	return nil
}

// ApplyClusterRoleToServiceAccount Creates cluster-role-binding as a link between role and service account
func (k *Kube) ApplyClusterRoleToServiceAccount(roleName string, serviceAccount *corev1.ServiceAccount, ownerReference []metav1.OwnerReference) error {
	rolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s", serviceAccount.Namespace, serviceAccount.Name),
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return k.ApplyClusterRoleBinding(rolebinding)
}

func CreateManageSecretRoleBinding(adGroups []string, role *auth.Role) *auth.RoleBinding {
	subjects := GetRoleBindingGroups(adGroups)
	roleName := role.ObjectMeta.Name

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   roleName,
			Labels: role.Labels,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: subjects,
	}

	return rolebinding
}
