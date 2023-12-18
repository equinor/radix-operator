package dnsalias

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// syncRbac Grants access to Radix DNSAlias to application admin and reader
func (s *syncer) syncRbac() error {
	appName := s.radixDNSAlias.Spec.AppName
	rr, err := s.kubeUtil.GetRegistration(appName)
	if err != nil {
		return err
	}
	appAdminSubjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}

	adminClusterRole := s.buildDNSAliasClusterRole(s.getClusterRoleNameForAdmin(), []string{"get", "list"})
	if err := s.kubeUtil.ApplyClusterRole(adminClusterRole); err != nil {
		return err
	}
	readerClusterRole := s.buildDNSAliasClusterRole(s.getClusterRoleNameForReader(), []string{"get", "list"})
	if err := s.kubeUtil.ApplyClusterRole(readerClusterRole); err != nil {
		return err
	}
	adminClusterRoleBinding := s.buildClusterRoleBinding(adminClusterRole, appAdminSubjects)
	if err := s.kubeUtil.ApplyClusterRoleBinding(adminClusterRoleBinding); err != nil {
		return err
	}
	appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	readerClusterRoleBinding := s.buildClusterRoleBinding(readerClusterRole, appReaderSubjects)
	return s.kubeUtil.ApplyClusterRoleBinding(readerClusterRoleBinding)
}

func (s *syncer) getClusterRoleNameForAdmin() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationAdminRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) getClusterRoleNameForReader() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationReaderRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) buildDNSAliasClusterRole(clusterRoleName string, verbs []string) *rbacv1.ClusterRole {
	return s.buildClusterRole(clusterRoleName, rbacv1.PolicyRule{APIGroups: []string{radixv1.SchemeGroupVersion.Group},
		Resources:     []string{radixv1.ResourceRadixDNSAliases, radixv1.ResourceRadixDNSAliasStatuses},
		ResourceNames: []string{s.radixDNSAlias.GetName()},
		Verbs:         verbs,
	})
}

func (s *syncer) buildClusterRole(clusterRoleName string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRole,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterRoleName,
			Labels:          s.getLabelsForDNSAliasRbac(),
			OwnerReferences: s.getOwnerReference(),
		},
		Rules: rules,
	}
}

func (s *syncer) buildClusterRoleBinding(clusterRole *rbacv1.ClusterRole, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterRole.Name,
			Labels:          s.getLabelsForDNSAliasRbac(),
			OwnerReferences: s.getOwnerReference(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     clusterRole.Name,
		},
		Subjects: subjects,
	}
}

func (s *syncer) getLabelsForDNSAliasRbac() labels.Set {
	return radixlabels.ForDNSAliasRbac(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.GetName())
}

func (s *syncer) deleteRbac() error {
	selectorForDNSAlias := s.getLabelsForDNSAliasRbac().String()
	clusterRoleBindings, err := s.kubeUtil.ListClusterRoleBindingsWithSelector(selectorForDNSAlias)
	if err != nil {
		return err
	}
	clusterRoles, err := s.kubeUtil.ListClusterRolesWithSelector(selectorForDNSAlias)
	if err != nil {
		return err
	}
	for _, binding := range clusterRoleBindings {
		if err := s.kubeUtil.DeleteClusterRoleBinding(binding.GetName()); err != nil {
			return err
		}
	}
	for _, role := range clusterRoles {
		if err := s.kubeUtil.DeleteClusterRole(role.GetName()); err != nil {
			return err
		}
	}
	return nil
}

func (s *syncer) getOwnerReference() []metav1.OwnerReference {
	return internal.GetOwnerReferences(s.radixDNSAlias)
}
