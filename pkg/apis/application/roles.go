package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) buildRRClusterRole(clusterRoleName string, verbs []string) *auth.ClusterRole {
	appName := app.registration.Name
	return app.buildClusterRole(clusterRoleName, auth.PolicyRule{APIGroups: []string{radix.GroupName},
		Resources:     []string{radix.ResourceRadixRegistrations},
		ResourceNames: []string{appName},
		Verbs:         verbs,
	})
}

func (app Application) buildRadixDNSAliasClusterRole(roleNamePrefix string) *auth.ClusterRole {
	clusterRoleName := fmt.Sprintf("%s-%s", roleNamePrefix, app.registration.Name)
	return app.buildClusterRole(clusterRoleName, auth.PolicyRule{APIGroups: []string{radix.GroupName},
		Resources: []string{radix.ResourceRadixDNSAliases},
		Verbs:     []string{"list"},
	})
}

func (app Application) buildClusterRole(clusterRoleName string, rules ...auth.PolicyRule) *auth.ClusterRole {
	logger.Debugf("Creating clusterrole config %s", clusterRoleName)
	clusterRole := &auth.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
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
	logger.Debugf("Done - creating clusterrole config %s", clusterRoleName)
	return clusterRole
}
