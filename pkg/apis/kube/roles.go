package kube

import (
	"fmt"

	"github.com/statoil/radix-operator/pkg/apis/utils"

	log "github.com/Sirupsen/logrus"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func RrUserRole(registration *radixv1.RadixRegistration) *auth.Role {
	appName := registration.Name
	roleName := fmt.Sprintf("operator-rr-%s", appName)
	return RrRole(registration, roleName, []string{"get", "list", "watch", "update", "patch", "delete"})
}

func RrPipelineRole(registration *radixv1.RadixRegistration) *auth.Role {
	appName := registration.Name
	roleName := fmt.Sprintf("radix-pipeline-%s", appName)
	return RrRole(registration, roleName, []string{"get"})
}

func RrRole(registration *radixv1.RadixRegistration, roleName string, verbs []string) *auth.Role {
	appName := registration.Name
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})

	ownerRef := GetOwnerReference(roleName, "RadixRegistration", registration.UID)

	logger.Infof("Creating role config %s", roleName)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{"radix.equinor.com"},
				Resources:     []string{"radixregistrations"},
				ResourceNames: []string{appName},
				Verbs:         verbs,
			},
		},
	}
	logger.Infof("Done - creating role config %s", roleName)

	return role
}

func roleAppAdminSecrets(registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent) *auth.Role {
	secretName := utils.GetComponentSecretName(component.Name)
	roleName := fmt.Sprintf("radix-app-adm-%s", component.Name)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixReg": registration.Name,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
				Verbs:         []string{"get", "list", "watch", "update", "patch", "delete"},
			},
		},
	}
	return role
}
