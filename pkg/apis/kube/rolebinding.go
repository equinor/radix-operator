package kube

import (
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
func GetRolebindingToRole(appName, roleName string, groups []string) *auth.RoleBinding {
	return GetRolebindingToRoleWithLabels(roleName, groups, map[string]string{
		RadixAppLabel: appName,
	})
}

// GetRolebindingToRoleWithLabels Get role binding object
func GetRolebindingToRoleWithLabels(roleName string, groups []string, labels map[string]string) *auth.RoleBinding {
	subjects := GetRoleBindingGroups(groups)
	return getRoleBindingForSubjects(roleName, "Role", subjects, labels)
}

// GetRolebindingToRoleWithLabelsForSubjects Get rolebinding object with subjects as input
func GetRolebindingToRoleWithLabelsForSubjects(roleName string, subjects []auth.Subject, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForSubjects(roleName, "Role", subjects, labels)
}

// GetRolebindingToClusterRole Get role binding object
func GetRolebindingToClusterRole(appName, roleName string, groups []string) *auth.RoleBinding {
	return GetRolebindingToClusterRoleWithLabels(roleName, groups, map[string]string{
		RadixAppLabel: appName,
	})
}

// GetRolebindingToClusterRoleForSubjects Get role binding object for list of subjects
func GetRolebindingToClusterRoleForSubjects(appName, roleName string, subjects []auth.Subject) *auth.RoleBinding {
	return GetRolebindingToClusterRoleForSubjectsWithLabels(appName, roleName, subjects, map[string]string{
		RadixAppLabel: appName,
	})
}

// GetRolebindingToClusterRoleForSubjectsWithLabels Get role binding object for list of subjects with labels set
func GetRolebindingToClusterRoleForSubjectsWithLabels(appName, roleName string, subjects []auth.Subject, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForSubjects(roleName, "ClusterRole", subjects, labels)
}

// GetRolebindingToClusterRoleWithLabels Get role binding object
func GetRolebindingToClusterRoleWithLabels(roleName string, groups []string, labels map[string]string) *auth.RoleBinding {
	subjects := GetRoleBindingGroups(groups)
	return getRoleBindingForSubjects(roleName, "ClusterRole", subjects, labels)
}

// GetRolebindingToRoleForSubjectsWithLabels Get role binding object for list of subjects with labels set
func GetRolebindingToRoleForSubjectsWithLabels(appName, roleName string, subjects []auth.Subject, labels map[string]string) *auth.RoleBinding {
	return getRoleBindingForSubjects(roleName, "Role", subjects, labels)
}

// GetRolebindingToRoleForServiceAccountWithLabels Get role binding object
func GetRolebindingToRoleForServiceAccountWithLabels(roleName, serviceAccountName, serviceAccountNamespace string, labels map[string]string) *auth.RoleBinding {
	subjects := []auth.Subject{
		auth.Subject{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		}}

	return getRoleBindingForSubjects(roleName, "Role", subjects, labels)
}

// GetRolebindingToClusterRoleForServiceAccountWithLabels Get role binding object
func GetRolebindingToClusterRoleForServiceAccountWithLabels(roleName, serviceAccountName, serviceAccountNamespace string, labels map[string]string) *auth.RoleBinding {
	subjects := []auth.Subject{
		auth.Subject{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		}}

	return getRoleBindingForSubjects(roleName, "ClusterRole", subjects, labels)
}

func getRoleBindingForSubjects(roleName, kind string, subjects []auth.Subject, labels map[string]string) *auth.RoleBinding {
	return &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   roleName,
			Labels: labels,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     kind,
			Name:     roleName,
		},
		Subjects: subjects,
	}
}

// ApplyRoleBinding Creates or updates role
func (k *Kube) ApplyRoleBinding(namespace string, role *auth.RoleBinding) error {
	logger.Debugf("Apply role binding %s", role.Name)
	oldRoleBinding, err := k.GetRoleBinding(namespace, role.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(role)
		if err != nil {
			return fmt.Errorf("Failed to create role binding object: %v", err)
		}

		log.Debugf("Created role binding: %s in namespace %s", createdRoleBinding.Name, namespace)
		return nil

	} else if err != nil {
		return fmt.Errorf("Failed to get role binding object: %v", err)
	}

	log.Debugf("Role binding object %s already exists in namespace %s, updating the object now", role.GetName(), namespace)

	newRoleBinding := oldRoleBinding.DeepCopy()
	newRoleBinding.ObjectMeta.OwnerReferences = role.ObjectMeta.OwnerReferences
	newRoleBinding.ObjectMeta.Labels = role.Labels
	newRoleBinding.Subjects = role.Subjects

	oldRoleBindingJSON, err := json.Marshal(oldRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal old role binding object: %v", err)
	}

	newRoleBindingJSON, err := json.Marshal(newRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal new role binding object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldRoleBindingJSON, newRoleBindingJSON, v1beta1.RoleBinding{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch role binding objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Patch(role.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch role binding object: %v", err)
		}
		log.Debugf("Patched role binding: %s in namespace %s", patchedRoleBinding.Name, namespace)
	} else {
		log.Debugf("No need to patch role binding: %s ", role.GetName())
	}

	return nil
}

// ApplyClusterRoleBinding Creates or updates cluster-role-binding
func (k *Kube) ApplyClusterRoleBinding(clusterrolebinding *auth.ClusterRoleBinding) error {
	logger = logger.WithFields(log.Fields{"clusterRoleBinding": clusterrolebinding.ObjectMeta.Name})
	logger.Debugf("Apply clusterrolebinding %s", clusterrolebinding.Name)
	oldClusterRoleBinding, err := k.getClusterRoleBinding(clusterrolebinding.Name)
	if err != nil && errors.IsNotFound(err) {
		createdClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Create(clusterrolebinding)
		if err != nil {
			return fmt.Errorf("Failed to create cluster role binding object: %v", err)
		}

		log.Debugf("Created cluster role binding: %s", createdClusterRoleBinding.Name)
		return nil

	} else if err != nil {
		return fmt.Errorf("Failed to get cluster role binding object: %v", err)
	}

	log.Debugf("Role binding object %s already exists, updating the object now", clusterrolebinding.GetName())

	newClusterRoleBinding := oldClusterRoleBinding.DeepCopy()
	newClusterRoleBinding.ObjectMeta.OwnerReferences = clusterrolebinding.OwnerReferences
	newClusterRoleBinding.ObjectMeta.Labels = clusterrolebinding.Labels
	newClusterRoleBinding.Subjects = clusterrolebinding.Subjects

	oldClusterRoleBindingJSON, err := json.Marshal(oldClusterRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal old cluster role binding object: %v", err)
	}

	newClusterRoleBindingJSON, err := json.Marshal(newClusterRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal new cluster role binding object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClusterRoleBindingJSON, newClusterRoleBindingJSON, v1beta1.ClusterRoleBinding{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch cluster role binding objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		patchedClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Patch(clusterrolebinding.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch cluster role binding object: %v", err)
		}
		log.Debugf("Patched cluster role binding: %s ", patchedClusterRoleBinding.Name)
	} else {
		log.Debugf("No need to patch cluster role binding: %s ", clusterrolebinding.GetName())
	}

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

// GetRoleBinding Gets rolebinding
func (k *Kube) GetRoleBinding(namespace, name string) (*auth.RoleBinding, error) {
	var role *auth.RoleBinding
	var err error

	if k.RoleBindingLister != nil {
		role, err = k.RoleBindingLister.RoleBindings(namespace).Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		role, err = k.kubeClient.RbacV1().RoleBindings(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return role, nil
}

// ListRoleBindings Lists role bindings from cache or from cluster
func (k *Kube) ListRoleBindings(namespace string) ([]*auth.RoleBinding, error) {
	return k.ListRoleBindingsWithSelector(namespace, nil)
}

// ListRoleBindingsWithSelector Lists role bindings from cache or from cluster using a selector
func (k *Kube) ListRoleBindingsWithSelector(namespace string, labelSelectorString *string) ([]*auth.RoleBinding, error) {
	var roleBindings []*auth.RoleBinding
	var err error

	if k.RoleBindingLister != nil {
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

		roleBindings, err = k.RoleBindingLister.RoleBindings(namespace).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := k.kubeClient.RbacV1().RoleBindings(namespace).List(listOptions)
		if err != nil {
			return nil, err
		}

		roleBindings = slice.PointersOf(list.Items).([]*auth.RoleBinding)
	}

	return roleBindings, nil
}

// ListClusterRoleBindings List cluster roles
func (k *Kube) ListClusterRoleBindings(namespace string) ([]*auth.ClusterRoleBinding, error) {
	var clusterRoleBindings []*auth.ClusterRoleBinding
	var err error

	if k.ClusterRoleBindingLister != nil {
		clusterRoleBindings, err = k.ClusterRoleBindingLister.List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := k.kubeClient.RbacV1().ClusterRoleBindings().List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		clusterRoleBindings = slice.PointersOf(list.Items).([]*auth.ClusterRoleBinding)
	}

	return clusterRoleBindings, nil
}

// DeleteClusterRoleBinding Deletes a clusterrolebinding
func (k *Kube) DeleteClusterRoleBinding(name string) error {
	_, err := k.getClusterRoleBinding(name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get clusterrolebinding object: %v", err)
	}
	err = k.kubeClient.RbacV1().ClusterRoleBindings().Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete clusterrolebinding object: %v", err)
	}
	return nil
}

// DeleteRoleBinding Deletes a rolebinding in a namespace
func (k *Kube) DeleteRoleBinding(namespace, name string) error {
	_, err := k.GetRoleBinding(namespace, name)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to get rolebinding object: %v", err)
	}
	err = k.kubeClient.RbacV1().RoleBindings(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete rolebinding object: %v", err)
	}
	return nil
}

func (k *Kube) getClusterRoleBinding(name string) (*auth.ClusterRoleBinding, error) {
	var clusterRoleBinding *auth.ClusterRoleBinding
	var err error

	if k.ClusterRoleBindingLister != nil {
		clusterRoleBinding, err = k.ClusterRoleBindingLister.Get(name)
		if err != nil {
			return nil, err
		}
	} else {
		clusterRoleBinding, err = k.kubeClient.RbacV1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return clusterRoleBinding, nil
}
