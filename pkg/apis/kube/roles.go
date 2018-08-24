package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	core "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *Kube) ApplyRole(namespace string, role *auth.Role) error {
	logger.Infof("Apply role %s", role.Name)
	_, err := k.kubeClient.RbacV1().Roles(namespace).Create(role)
	if errors.IsAlreadyExists(err) {
		logger.Infof("Role %s already exists", role.Name)
		return nil
	}

	if err != nil {
		logger.Infof("Creating role %s failed: %v", role.Name, err)
		return err
	}
	logger.Infof("Created role %s in %s", role.Name, namespace)
	return nil
}

func RdRole(radixDeploy *v1.RadixDeployment, adGroups []string) *auth.Role {
	appName := radixDeploy.Spec.AppName
	roleName := fmt.Sprintf("operator-rd-%s", appName)
	ownerReference := GetOwnerReference(radixDeploy.Name, "RadixDeployment", radixDeploy.UID)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixDeploy": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerReference,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{"radix.equinor.com"},
				Resources:     []string{"RadixDeployment"},
				ResourceNames: []string{appName},
				Verbs:         []string{"get", "delete"},
			},
		},
	}
	return role
}

func RrRole(registration *radixv1.RadixRegistration, brigadeProject *core.Secret) *auth.Role {
	logger = logger.WithFields(log.Fields{"registrationName": registration.ObjectMeta.Name, "registrationNamespace": registration.ObjectMeta.Namespace})

	appName := registration.Name
	roleName := fmt.Sprintf("operator-rr-%s", appName)
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
				Verbs:         []string{"get", "update", "patch", "delete"},
			},
			{
				APIGroups:     []string{"*"},
				Resources:     []string{"secrets"},
				ResourceNames: []string{brigadeProject.Name},
				Verbs:         []string{"get", "watch"},
			},
		},
	}
	logger.Infof("Done - creating role config %s", roleName)

	return role
}

func BrigadeRole(brigadeBuildId string, radixApplication *v1.RadixApplication, owner metav1.OwnerReference) *auth.Role {
	appName := radixApplication.Name
	roleName := fmt.Sprintf("radix-brigade-%s-%s", appName, brigadeBuildId)
	resourceNames := getResourceNames(brigadeBuildId, radixApplication.Spec.Components, radixApplication.Spec.Environments)

	logger.Infof("Creating role config %s", roleName)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixBrigade": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{"*"},
				Resources:     []string{"secrets", "pods", "pods/log"},
				ResourceNames: resourceNames,
				Verbs:         []string{"get"},
			},
		},
	}
	logger.Infof("Done - creating role config %s", roleName)

	return role
}

func getResourceNames(brigadeBuildId string, components []v1.RadixComponent, environments []v1.Environment) []string {
	resourceNames := []string{
		fmt.Sprintf("brigade-worker-%s", brigadeBuildId),
		fmt.Sprintf("config-%s", brigadeBuildId),
		fmt.Sprintf("rbac-%s", brigadeBuildId),
	}

	for _, component := range components {
		resourceNames = append(resourceNames, fmt.Sprintf("build-%s-%s", component.Name, brigadeBuildId))
		for _, env := range environments {
			resourceNames = append(resourceNames, fmt.Sprintf("deploy-%s-%s-%s", env.Name, component.Name, brigadeBuildId))
		}
	}
	return resourceNames
}
