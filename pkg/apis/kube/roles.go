package kube

import (
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (k *Kube) ApplyRole(namespace string, role *auth.Role) error {
	logger.Infof("Apply role %s", role.Name)
	_, err := k.kubeClient.RbacV1().Roles(namespace).Create(role)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Role %s already exists. Updating", role.Name)
		_, err = k.kubeClient.RbacV1().Roles(namespace).Update(role)
	}

	if err != nil {
		logger.Infof("Saving role %s failed: %v", role.Name, err)
		return err
	}
	logger.Infof("Created role %s in %s", role.Name, namespace)
	return nil
}
