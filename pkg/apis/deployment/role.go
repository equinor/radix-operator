package deployment

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) garbageCollectRolesNoLongerInSpecForComponent(ctx context.Context, component radixv1.RadixCommonDeployComponent) error {
	labelSelector := getComponentSecretRbaclabels(deploy.registration.Name, component.GetName()).String()
	roles, err := deploy.kubeutil.ListRolesWithSelector(ctx, deploy.radixDeployment.GetNamespace(), labelSelector)
	if err != nil {
		return err
	}

	if len(roles) > 0 {
		for _, role := range roles {
			err = deploy.kubeclient.RbacV1().Roles(deploy.radixDeployment.GetNamespace()).Delete(ctx, role.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
