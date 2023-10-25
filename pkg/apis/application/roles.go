package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) rrPipelineClusterRole(roleNamePrefix string) *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name
	clusterroleName := fmt.Sprintf("%s-%s", roleNamePrefix, appName)
	return app.rrClusterRole(clusterroleName, []string{"get"})
}

func (app Application) rrClusterRole(clusterroleName string, verbs []string) *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name

	ownerRef := app.getOwnerReference()

	logger.Debugf("Creating clusterrole config %s", clusterroleName)

	clusterrole := &auth.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       k8s.KindClusterRole,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerRef,
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{radix.GroupName},
				Resources:     []string{"radixregistrations"},
				ResourceNames: []string{appName},
				Verbs:         verbs,
			},
		},
	}
	logger.Debugf("Done - creating clusterrole config %s", clusterroleName)

	return clusterrole
}
