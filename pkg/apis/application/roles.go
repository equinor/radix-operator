package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) rrUserClusterRole(clusterRoleName string, verbs []string) *auth.ClusterRole {
	return app.rrClusterrole(clusterRoleName, verbs)
}

func (app Application) rrPipelineClusterRole(roleNamePrefix string) *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name
	clusterroleName := fmt.Sprintf("%s-%s", roleNamePrefix, appName)
	return app.rrClusterrole(clusterroleName, []string{"get"})
}

func (app Application) rrClusterrole(clusterroleName string, verbs []string) *auth.ClusterRole {
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
				APIGroups:     []string{"radix.equinor.com"},
				Resources:     []string{"radixregistrations"},
				ResourceNames: []string{appName},
				Verbs:         verbs,
			},
		},
	}
	logger.Debugf("Done - creating clusterrole config %s", clusterroleName)

	return clusterrole
}

func roleAppAdminMachineUserToken(appName string, serviceAccount *corev1.ServiceAccount) *auth.Role {
	return kube.CreateManageSecretRole(appName, fmt.Sprintf("%s-%s", serviceAccount.Name, "token"), []string{serviceAccount.Secrets[0].Name}, nil)
}
