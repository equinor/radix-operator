package deployment

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) grantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent, secrets []string) error {
	if len(secrets) <= 0 {
		err := deploy.garbageCollectRoleBindingsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}

		return nil
	}

	role := roleAppAdminSecrets(registration, component, secrets)
	err := deploy.kubeutil.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminSecrets(registration, role)
	return deploy.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpecForComponent(component *v1.RadixDeployComponent) error {
	roleBindings, err := deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: getLabelSelectorForComponent(*component),
	})
	if err != nil {
		return err
	}

	if len(roleBindings.Items) > 0 {
		for n := range roleBindings.Items {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(roleBindings.Items[n].Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpec() error {
	roleBindings, err := deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, exisitingComponent := range roleBindings.Items {
		garbageCollect := true
		exisitingComponentName, exists := exisitingComponent.ObjectMeta.Labels[kube.RadixComponentLabel]

		if !exists {
			continue
		}

		for _, component := range deploy.radixDeployment.Spec.Components {
			if strings.EqualFold(component.Name, exisitingComponentName) {
				garbageCollect = false
				break
			}
		}

		if garbageCollect {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(exisitingComponent.Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func rolebindingAppAdminSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	adGroups, _ := application.GetAdGroups(registration)
	subjects := kube.GetRoleBindingGroups(adGroups)
	roleName := role.ObjectMeta.Name

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   roleName,
			Labels: role.Labels,
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
