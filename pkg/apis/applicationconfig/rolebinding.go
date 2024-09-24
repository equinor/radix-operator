package applicationconfig

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	auth "k8s.io/api/rbac/v1"
)

func rolebindingAppReaderToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	subjects := utils.GetAppReaderRbacSubjects(registration)
	roleName := role.ObjectMeta.Name
	return kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
}
func rolebindingAppAdminToBuildSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	subjects, _ := utils.GetAppAdminRbacSubjects(registration)
	roleName := role.ObjectMeta.Name
	return kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
}

func rolebindingPipelineToRole(role *auth.Role) *auth.RoleBinding {
	roleName := role.ObjectMeta.Name
	return kube.GetRolebindingToRoleForServiceAccountWithLabels(roleName, defaults.PipelineServiceAccountName, role.Namespace, role.Labels)
}
