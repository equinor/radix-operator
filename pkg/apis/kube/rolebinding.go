package kube

import (
	"fmt"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrantAppAdminAccessToNs Grant access to environment namespace
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) GrantAppAdminAccessToNs(namespace string, registration *radixv1.RadixRegistration) error {
	subjects := GetRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin-envs"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: registration.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: subjects,
	}

	return k.ApplyRoleBinding(namespace, roleBinding)
}

// GrantAppAdminAccessToRuntimeSecrets Grants access to runtime secrets in environment namespace
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) GrantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent) error {
	if component.Secrets == nil || len(component.Secrets) <= 0 {
		return nil
	}

	role := roleAppAdminSecrets(registration, component)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminSecrets(registration, role)

	return k.ApplyRoleBinding(namespace, rolebinding)
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

// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) ApplyClusterRoleToServiceAccount(roleName string, registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	// ownerReference := GetOwnerReferenceOfRegistration(registration) // TODO! reference into app package, shouldnt

	rolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s", serviceAccount.Namespace, serviceAccount.Name),
			OwnerReferences: nil,
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

func rolebindingAppAdminSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	subjects := GetRoleBindingGroups(registration.Spec.AdGroups)
	roleName := role.ObjectMeta.Name

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: registration.Name,
			},
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
