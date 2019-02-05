package kube

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (k *Kube) ApplyRoleBinding(namespace string, rolebinding *auth.RoleBinding) error {
	logger = logger.WithFields(log.Fields{"roleBinding": rolebinding.ObjectMeta.Name})

	logger.Infof("Apply rolebinding %s", rolebinding.Name)

	_, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		_, err = k.kubeClient.RbacV1().RoleBindings(namespace).Update(rolebinding)
		logger.Infof("Rolebinding %s already exists. Updating", rolebinding.Name)
	}

	if err != nil {
		logger.Errorf("Failed to save roleBinding in [%s]: %v", namespace, err)
		return err
	}

	logger.Infof("Created roleBinding %s in %s", rolebinding.Name, namespace)
	return nil
}

func (k *Kube) ApplyClusterRoleBinding(rolebinding *auth.ClusterRoleBinding) error {
	logger = logger.WithFields(log.Fields{"clusterRoleBinding": rolebinding.ObjectMeta.Name})

	logger.Infof("Apply clusterrolebinding %s", rolebinding.Name)

	_, err := k.kubeClient.RbacV1().ClusterRoleBindings().Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		logger.Infof("ClusterRolebinding %s already exists", rolebinding.Name)
		return nil
	}

	if err != nil {
		logger.Errorf("Failed to create clusterRoleBinding: %v", err)
		return err
	}

	logger.Infof("Created clusterRoleBinding %s", rolebinding.Name)
	return nil
}

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
