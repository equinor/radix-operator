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

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixReg":               registration.Name, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       registration.Name,
				kube.RadixComponentLabel: component.Name,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: secrets,
				Verbs:         []string{"get", "list", "watch", "update", "patch", "delete"},
			},
		},
	}
	return role
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
