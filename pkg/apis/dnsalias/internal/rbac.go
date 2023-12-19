package internal

import (
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildClusterRoleBinding Build a cluster role binding
func BuildClusterRoleBinding(clusterRoleName string, subjects []rbacv1.Subject, radixDNSAlias *radixv1.RadixDNSAlias) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterRoleName,
			Labels:          radixlabels.ForDNSAliasRbac(radixDNSAlias.Spec.AppName),
			OwnerReferences: GetOwnerReferences(radixDNSAlias, false),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     clusterRoleName,
		},
		Subjects: subjects,
	}
}
