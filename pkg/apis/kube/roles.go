package kube

import (
	"fmt"

	"github.com/statoil/radix-operator/pkg/apis/radix/v1"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *Kube) ApplyRole(namespace string, role *auth.Role) error {
	log.Infof("Apply role %s", role.Name)
	_, err := k.kubeClient.RbacV1().Roles(namespace).Create(role)
	if errors.IsAlreadyExists(err) {
		log.Infof("Role %s already exists", role.Name)
		return nil
	}

	if err != nil {
		log.Infof("Creating role %s failed: %v", role.Name, err)
		return err
	}
	log.Infof("Created role %s in %s", role.Name, namespace)
	return nil
}

func RdRole(radixDeploy *v1.RadixDeployment, adGroups []string) *auth.Role {
	appName := radixDeploy.Name
	roleName := fmt.Sprintf("operator-%s", appName)
	ownerReference := GetOwnerReference(appName, radixDeploy.Kind, radixDeploy.UID)

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

func RrRole(registration *radixv1.RadixRegistration) *auth.Role {
	appName := registration.Name
	roleName := fmt.Sprintf("operator-%s", appName)
	ownerRef := GetOwnerReference(roleName, "RadixRegistration", registration.UID)

	log.Infof("Creating role config %s", roleName)

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
		},
	}
	log.Infof("Done - creating role config %s", roleName)

	return role
}

func BrigadeRole(brigadeBuildId string, radixAppliation *v1.RadixApplication, owner metav1.OwnerReference) *auth.Role {
	appName := radixAppliation.Name
	roleName := fmt.Sprintf("radix-brigade-%s-%s", appName, brigadeBuildId)
	resourceNames := getResourceNames(brigadeBuildId, radixAppliation.Spec.Components, radixAppliation.Spec.Environments)

	log.Infof("Creating role config %s", roleName)

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
	log.Infof("Done - creating role config %s", roleName)

	return role
}

func getResourceNames(brigadeBuildId string, components []v1.RadixComponent, environments []v1.Environment) []string {
	resourceNames := []string{
		fmt.Sprintf("brigade-worker-%s", brigadeBuildId),
		fmt.Sprintf("config-%s", brigadeBuildId),
		fmt.Sprintf("rbac-%s", brigadeBuildId),
	}

	for _, env := range environments {
		resourceNames = append(resourceNames, fmt.Sprintf("build-%s-%s", env.Name, brigadeBuildId))
		for _, component := range components {
			resourceNames = append(resourceNames, fmt.Sprintf("deploy-%s-%s-%s", env.Name, component.Name, brigadeBuildId))
		}
	}
	return resourceNames
}
