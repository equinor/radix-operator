package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *Application) buildRRClusterRole(clusterRoleName string, verbs []string) *rbacv1.ClusterRole {
	appName := app.registration.Name
	return app.buildClusterRole(clusterRoleName, rbacv1.PolicyRule{APIGroups: []string{v1.SchemeGroupVersion.Group},
		Resources:     []string{v1.ResourceRadixRegistrations},
		ResourceNames: []string{appName},
		Verbs:         verbs,
	})
}

func (app *Application) buildRadixDNSAliasClusterRole(roleNamePrefix string) *rbacv1.ClusterRole {
	clusterRoleName := fmt.Sprintf("%s-%s", roleNamePrefix, app.registration.Name)
	return app.buildClusterRole(clusterRoleName, rbacv1.PolicyRule{APIGroups: []string{v1.SchemeGroupVersion.Group},
		Resources: []string{v1.ResourceRadixDNSAliases},
		Verbs:     []string{"list"},
	})
}

func (app *Application) buildClusterRole(clusterRoleName string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	app.logger.Debug().Msgf("Creating clusterrole config %s", clusterRoleName)
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRole,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: app.registration.Name,
			},
			OwnerReferences: app.getOwnerReference(),
		},
		Rules: rules,
	}
	app.logger.Debug().Msgf("Done - creating clusterrole config %s", clusterRoleName)
	return clusterRole
}
