package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"

	radixv1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *Kube) CreateRoleBindings(app *radixv1.RadixApplication) {
	for _, env := range app.Spec.Environments {
		for _, auth := range env.Authorization {
			k.CreateRoleBinding(app.Name, fmt.Sprintf("%s-%s", app.Name, env.Name), auth.Role, auth.Groups)
		}
	}
}

func (k *Kube) CreateRoleBinding(appName, namespace, clusterrole string, groups []string) error {
	subjects := []auth.Subject{}
	for _, group := range groups {
		subjects = append(subjects, auth.Subject{
			Kind:     "Group",
			Name:     group,
			APIGroup: "rbac.authorization.k8s.io",
		})
	}

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

	_, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if err != nil {
		log.Errorf("Failed to create rolebinding in [%s] for %s: %v", namespace, appName, err)
		return err
	}
	log.Infof("Created rolebinding %s in %s", rolebinding.Name, namespace)
	return nil
}
