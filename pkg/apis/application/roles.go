package application

import (
	"fmt"

	auth "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) rrUserRole() *auth.Role {
	registration := app.registration
	appName := registration.Name
	roleName := fmt.Sprintf("operator-rr-%s", appName)
	return app.rrRole(roleName, []string{"get", "list", "watch", "update", "patch", "delete"})
}

func (app Application) rrPipelineRole() *auth.Role {
	registration := app.registration
	appName := registration.Name
	roleName := fmt.Sprintf("radix-pipeline-%s", appName)
	return app.rrRole(roleName, []string{"get"})
}

func (app Application) rrRole(roleName string, verbs []string) *auth.Role {
	registration := app.registration
	appName := registration.Name

	ownerRef := app.getOwnerReference()

	logger.Debugf("Creating role config %s", roleName)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
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
	logger.Debugf("Done - creating role config %s", roleName)

	return role
}
