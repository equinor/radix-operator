package deployment

import (
	"strings"

	"github.com/equinor/radix-operator/pkg/apis/application"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	auth "k8s.io/api/rbac/v1"
	labelHelpers "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (deploy *Deployment) grantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent, secrets []string) error {
	if len(secrets) <= 0 {
		err := deploy.garbageCollectRoleBindingsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}

		err = deploy.garbageCollectRolesNoLongerInSpecForComponent(component)
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
	labelSelector := getLabelSelectorForComponent(*component)
	roleBindings, err := deploy.listRoleBindingsWithSelector(&labelSelector)
	if err != nil {
		return err
	}

	if len(roleBindings) > 0 {
		for n := range roleBindings {
			err = deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).Delete(roleBindings[n].Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectRoleBindingsNoLongerInSpec() error {
	roleBindings, err := deploy.listRoleBindings()
	if err != nil {
		return err
	}

	for _, exisitingComponent := range roleBindings {
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

func (deploy *Deployment) listRoleBindings() ([]*auth.RoleBinding, error) {
	return deploy.listRoleBindingsWithSelector(nil)
}

func (deploy *Deployment) listRoleBindingsWithSelector(labelSelectorString *string) ([]*auth.RoleBinding, error) {
	var roleBindings []*auth.RoleBinding
	var err error

	if deploy.kubeutil.RoleBindingLister != nil {
		var selector labels.Selector
		if labelSelectorString != nil {
			labelSelector, err := labelHelpers.ParseToLabelSelector(*labelSelectorString)
			if err != nil {
				return nil, err
			}

			selector, err = labelHelpers.LabelSelectorAsSelector(labelSelector)
			if err != nil {
				return nil, err
			}

		} else {
			selector = labels.NewSelector()
		}

		roleBindings, err = deploy.kubeutil.RoleBindingLister.RoleBindings(deploy.radixDeployment.GetNamespace()).List(selector)
		if err != nil {
			return nil, err
		}
	} else {
		listOptions := metav1.ListOptions{}
		if labelSelectorString != nil {
			listOptions.LabelSelector = *labelSelectorString
		}

		list, err := deploy.kubeclient.RbacV1().RoleBindings(deploy.radixDeployment.GetNamespace()).List(listOptions)
		if err != nil {
			return nil, err
		}

		roleBindings = slice.PointersOf(list.Items).([]*auth.RoleBinding)
	}

	return roleBindings, nil
}
