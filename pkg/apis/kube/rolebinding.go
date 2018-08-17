package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	core "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (k *Kube) ApplyRbacRadixApplication(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		for _, auth := range env.Authorization {
			namespace := fmt.Sprintf("%s-%s", app.Name, env.Name)
			rolebinding := appRoleBinding(app.Name, auth.Role, auth.Groups)
			err := k.ApplyRoleBinding(namespace, rolebinding)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *Kube) ApplyRbacRadixRegistration(registration *radixv1.RadixRegistration, brigadeProject *core.Secret) error {
	namespace := "default"

	role := RrRole(registration, brigadeProject)
	rolebinding := rrRoleBinding(registration, role)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	return k.ApplyRoleBinding(namespace, rolebinding)
}

func (k *Kube) ApplyRoleBinding(namespace string, rolebinding *auth.RoleBinding) error {
	log.Infof("Apply rolebinding %s", rolebinding.Name)
	_, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		log.Infof("Rolebinding %s already exists", rolebinding.Name)
		return nil
	}

	if err != nil {
		log.Errorf("Failed to create rolebinding in [%s]: %v", namespace, err)
		return err
	}

	log.Infof("Created rolebinding %s in %s", rolebinding.Name, namespace)
	return nil
}

func GetOwnerReference(name, kind string, uid types.UID) metav1.OwnerReference {
	trueVar := true
	ownerRef := metav1.OwnerReference{
		APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
		Kind:       kind,
		Name:       name,
		UID:        uid,
		Controller: &trueVar,
	}
	log.Infof("owner reference uid: %v, name: %s, kind: %s", uid, name, kind)
	return ownerRef
}

func BrigadeRoleBinding(appName, roleName string, adGroups []string, owner metav1.OwnerReference) *auth.RoleBinding {
	subjects := getRoleBindingGroups(adGroups)
	roleBindingName := roleName

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixBrigade": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: subjects,
	}

	log.Infof("Done - create rolebinding config %s", roleBindingName)

	return rolebinding
}

func RdRoleBinding(radixDeploy *radixv1.RadixDeployment, roleName string, adGroups []string) *auth.RoleBinding {
	appName := radixDeploy.Spec.AppName
	roleBindingName := roleName
	ownerReference := GetOwnerReference(radixDeploy.Name, "RadixDeployment", radixDeploy.UID)
	subjects := getRoleBindingGroups(adGroups)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixDeploy": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerReference,
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

func appRoleBinding(appName, clusterrole string, groups []string) *auth.RoleBinding {
	subjects := getRoleBindingGroups(groups)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("operator-%s-%s", appName, clusterrole),
			Labels: map[string]string{
				"radixApp": appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterrole,
		},
		Subjects: subjects,
	}

	return rolebinding
}

func rrRoleBinding(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	appName := registration.Name
	roleBindingName := role.Name
	log.Infof("Create rolebinding config %s", roleBindingName)

	ownerReference := GetOwnerReference(roleBindingName, "RadixRegistration", registration.UID)
	subjects := getRoleBindingGroups(registration.Spec.AdGroups)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				ownerReference,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: subjects,
	}

	log.Infof("Done - create rolebinding config %s", roleBindingName)

	return rolebinding
}

func getRoleBindingGroups(groups []string) []auth.Subject {
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
