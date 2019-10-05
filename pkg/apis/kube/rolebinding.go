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

// ApplyRoleBinding Creates or updates role
func (k *Kube) ApplyRoleBinding(namespace string, role *auth.RoleBinding) error {
	logger.Debugf("Apply role %s", role.Name)
	oldRoleBinding, err := k.getRoleBinding(namespace, role.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(role)
		if err != nil {
			return fmt.Errorf("Failed to create RoleBinding object: %v", err)
		}

		log.Debugf("Created RoleBinding: %s in namespace %s", createdRoleBinding.Name, namespace)
		return nil
	}

	log.Debugf("RoleBinding object %s already exists in namespace %s, updating the object now", role.GetName(), namespace)

	newRoleBinding := oldRoleBinding.DeepCopy()
	newRoleBinding.ObjectMeta.OwnerReferences = role.ObjectMeta.OwnerReferences
	newRoleBinding.ObjectMeta.Labels = role.Labels
	newRoleBinding.Subjects = role.Subjects

	oldRoleBindingJSON, err := json.Marshal(oldRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal old role object: %v", err)
	}

	newRoleBindingJSON, err := json.Marshal(newRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal new role object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldRoleBindingJSON, newRoleBindingJSON, v1beta1.RoleBinding{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch role objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		log.Debugf("#########YALLA##########Patch role with %s", string(patchBytes))
		patchedRoleBinding, err := k.kubeClient.RbacV1().RoleBindings(namespace).Patch(role.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch role object: %v", err)
		}
		log.Debugf("#########YALLA##########Patched role: %s in namespace %s", patchedRoleBinding.Name, namespace)
	} else {
		log.Debugf("#########YALLA##########No need to patch role: %s ", role.GetName())
	}

	return nil
}

// ApplyClusterRoleBinding Creates or updates cluster-role
func (k *Kube) ApplyClusterRoleBinding(clusteroleBinding *auth.ClusterRoleBinding) error {
	logger.Debugf("Apply clusteroleBinding %s", clusteroleBinding.Name)
	oldClusterRoleBinding, err := k.getClusterRoleBinding(clusteroleBinding.GetName())
	if err != nil && errors.IsNotFound(err) {
		createdClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Create(clusteroleBinding)
		if err != nil {
			return fmt.Errorf("Failed to create ClusterRoleBinding object: %v", err)
		}

		log.Debugf("Created ClusterRoleBinding: %s", createdClusterRoleBinding.Name)
		return nil
	}

	log.Debugf("ClusterRoleBinding object %s already exists, updating the object now", clusteroleBinding.GetName())

	newClusterRoleBinding := oldClusterRoleBinding.DeepCopy()
	newClusterRoleBinding.ObjectMeta.OwnerReferences = clusteroleBinding.ObjectMeta.OwnerReferences
	newClusterRoleBinding.ObjectMeta.Labels = clusteroleBinding.Labels
	newClusterRoleBinding.Subjects = clusteroleBinding.Subjects

	oldClusterRoleBindingJSON, err := json.Marshal(oldClusterRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal old clusteroleBinding object: %v", err)
	}

	newClusterRoleBindingJSON, err := json.Marshal(newClusterRoleBinding)
	if err != nil {
		return fmt.Errorf("Failed to marshal new clusteroleBinding object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldClusterRoleBindingJSON, newClusterRoleBindingJSON, v1beta1.ClusterRoleBinding{})
	if err != nil {
		return fmt.Errorf("Failed to create two way merge patch clusteroleBinding objects: %v", err)
	}

	if !isEmptyPatch(patchBytes) {
		log.Debugf("#########YALLA##########Patch clusteroleBinding with %s", string(patchBytes))
		patchedClusterRoleBinding, err := k.kubeClient.RbacV1().ClusterRoleBindings().Patch(clusteroleBinding.GetName(), types.StrategicMergePatchType, patchBytes)
		if err != nil {
			return fmt.Errorf("Failed to patch clusteroleBinding object: %v", err)
		}
		log.Debugf("#########YALLA##########Patched clusteroleBinding: %s", patchedClusterRoleBinding.Name)
	} else {
		log.Debugf("#########YALLA##########No need to patch clusteroleBinding: %s ", clusteroleBinding.GetName())
	}

	return nil
}

// ListRoleBindings List roles
func (k *Kube) ListRoleBindings(namespace string) ([]*auth.RoleBinding, error) {
	var roles []*auth.RoleBinding
	var err error

	if k.RoleBindingLister != nil {
		roles, err = k.RoleBindingLister.RoleBindings(namespace).List(labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		list, err := k.kubeClient.RbacV1().RoleBindings(namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		roles = slice.PointersOf(list.Items).([]*auth.RoleBinding)
	}

	return roles, nil
}

func (k *Kube) getRoleBinding(namespace, name string) (*auth.RoleBinding, error) {
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
			Kind:     "ClusterRoleBinding",
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
