package kube

import (
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyRole Creates or updates role
func (k *Kube) ApplyRole(namespace string, role *auth.Role) error {
	logger.Debugf("Apply role %s", role.Name)
	_, err := k.kubeClient.RbacV1().Roles(namespace).Create(role)
	if errors.IsAlreadyExists(err) {
		logger.Debugf("Role %s already exists. Updating", role.Name)
		_, err = k.kubeClient.RbacV1().Roles(namespace).Update(role)
	}

	if err != nil {
		logger.Debugf("Saving role %s failed: %v", role.Name, err)
		return err
	}
	logger.Debugf("Created role %s in %s", role.Name, namespace)
	return nil
}

// ApplyClusterRole Creates or updates cluster-role
func (k *Kube) ApplyClusterRole(clusterrole *auth.ClusterRole) error {
	logger.Debugf("Apply clusterrole %s", clusterrole.Name)
	_, err := k.kubeClient.RbacV1().ClusterRoles().Create(clusterrole)
	if errors.IsAlreadyExists(err) {
		logger.Debugf("Clusterrole %s already exists. Updating", clusterrole.Name)
		_, err = k.kubeClient.RbacV1().ClusterRoles().Update(clusterrole)
	}

	if err != nil {
		logger.Debugf("Saving clusterrole %s failed: %v", clusterrole.Name, err)
		return err
	}
	logger.Debugf("Created clusterrole %s", clusterrole.Name)
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
