package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (k *Kube) CreateRoleBindings(app *radixv1.RadixApplication) error {
	for _, env := range app.Spec.Environments {
		for _, auth := range env.Authorization {
			err := k.CreateRoleBinding(app.Name, fmt.Sprintf("%s-%s", app.Name, env.Name), auth.Role, auth.Groups)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *Kube) CreateRoleBinding(appName, namespace, clusterrole string, groups []string) error {
	subjects := GetRoleBindingGroups(groups)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", appName, clusterrole),
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

	return k.ApplyRoleBinding(namespace, rolebinding)
}

func (k *Kube) SetAccessOnRadixRegistration(registration *radixv1.RadixRegistration) error {
	namespace := "default"

	role := getRoleFor(registration)
	rolebinding := getRoleBindingFor(registration, role)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	return k.ApplyRoleBinding(namespace, rolebinding)
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

func getRoleFor(registration *radixv1.RadixRegistration) *auth.Role {
	appName := registration.Name
	roleName := fmt.Sprintf("operator-%s", appName)
	ownerRef := getOwnerReference(roleName, "RadixRegistration", registration.UID)

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

func getRoleBindingFor(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	appName := registration.Name
	roleBindingName := fmt.Sprintf("%s-binding", role.Name)
	log.Infof("Create rolebinding config %s", roleBindingName)

	ownerReference := getOwnerReference(roleBindingName, "RadixRegistration", registration.UID)
	subjects := GetRoleBindingGroups(registration.Spec.AdGroups)

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

func getOwnerReference(name, kind string, uid types.UID) metav1.OwnerReference {
	trueVar := true
	ownerRef := metav1.OwnerReference{
		APIVersion: "radix.equinor.com/v1", //need to hardcode these values for now - seems they are missing from the CRD in k8s 1.8
		Kind:       kind,
		Name:       name,
		UID:        uid,
		Controller: &trueVar,
	}
	return ownerRef
}
