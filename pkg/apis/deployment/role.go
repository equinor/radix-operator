package deployment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func roleAppAdminSecrets(registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent, secrets []string) *auth.Role {
	roleName := fmt.Sprintf("radix-app-adm-%s", component.Name)
	return kube.CreateManageSecretRole(registration.Name, roleName, secrets, &map[string]string{kube.RadixComponentLabel: component.Name})
}

func (deploy *Deployment) garbageCollectRolesNoLongerInSpecForComponent(component *v1.RadixDeployComponent) error {
	roles, err := deploy.kubeclient.RbacV1().Roles(deploy.radixDeployment.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: getLabelSelectorForComponent(*component),
	})
	if err != nil {
		return err
	}

	if len(roles.Items) > 0 {
		for n := range roles.Items {
			err = deploy.kubeclient.RbacV1().Roles(deploy.radixDeployment.GetNamespace()).Delete(roles.Items[n].Name, &metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
