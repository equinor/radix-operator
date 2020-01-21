package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	auth "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) rrUserClusterRole() *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name
	clusterroleName := fmt.Sprintf("radix-platform-user-rr-%s", appName)
	return app.rrClusterrole(clusterroleName, []string{"get", "list", "watch", "update", "patch", "delete"})
}

func (app Application) rrPipelineClusterRole() *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name
	clusterroleName := fmt.Sprintf("radix-pipeline-rr-%s", appName)
	return app.rrClusterrole(clusterroleName, []string{"get"})
}

func (app Application) rrClusterrole(clusterroleName string, verbs []string) *auth.ClusterRole {
	registration := app.registration
	appName := registration.Name

	ownerRef := app.getOwnerReference()

	logger.Debugf("Creating clusterrole config %s", clusterroleName)

	clusterrole := &auth.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleName,
			Labels: map[string]string{
				"radixReg": appName,
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

func (app Application) configToMapRunnerRole() *auth.Role {
	return &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.ConfigToMapRunnerRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: app.registration.Name,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create"},
			},
		},
	}
}

func roleAppAdminMachineUserToken(appName string, machineUserTokenSecretName string) *auth.Role {
	return kube.CreateManageSecretRole(appName, getAppAdminRoleNameToMachineUserToken(machineUserTokenSecretName), []string{machineUserTokenSecretName}, nil)
}

func getAppAdminRoleNameToMachineUserToken(machineUserTokenSecretName string) string {
	return fmt.Sprintf("%s-%s", defaults.AppAdminRoleName, machineUserTokenSecretName)
}
