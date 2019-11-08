package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func roleAppAdminBuildSecrets(registration *radixv1.RadixRegistration, buildSecretName string) *auth.Role {
	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: buildSecretName,
			Labels: map[string]string{
				kube.RadixAppLabel: registration.Name,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{buildSecretName},
				Verbs:         []string{"get", "list", "watch", "update", "patch"},
			},
		},
	}
	return role
}

func garbageCollectAppAdminRoleToBuildSecrets(kubeclient kubernetes.Interface, namespace, name string) error {
	role, err := kubeclient.RbacV1().Roles(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !errors.IsNotFound(err) && role != nil {
		err := kubeclient.RbacV1().Roles(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
