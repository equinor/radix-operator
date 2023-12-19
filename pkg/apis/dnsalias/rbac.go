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
	adminRoleName := s.getClusterRoleNameForAdmin()
	readerRoleName := s.getClusterRoleNameForReader()
	if err := s.syncClusterRoles(adminRoleName, readerRoleName); err != nil {
		return err
	}
	return s.syncClusterRoleBindings(adminRoleName, readerRoleName)
}

func (s *syncer) syncClusterRoleBindings(adminRoleName, readerRoleName string) error {
	rr, err := s.kubeUtil.GetRegistration(s.radixDNSAlias.Spec.AppName)
	if err != nil {
		return err
	}
	if err := s.syncAdminClusterRoleBindings(adminRoleName, rr); err != nil {
		return err
	}
	return s.syncReaderClusterRoleBindings(readerRoleName, rr)
}

func (s *syncer) syncAdminClusterRoleBindings(adminRoleName string, rr *radixv1.RadixRegistration) error {
	appAdminSubjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}
	if len(appAdminSubjects) > 0 {
		adminClusterRoleBinding := internal.BuildClusterRoleBinding(adminRoleName, appAdminSubjects, s.radixDNSAlias)
		return s.kubeUtil.ApplyClusterRoleBinding(adminClusterRoleBinding)
	}
	return s.kubeUtil.DeleteClusterRoleBinding(adminRoleName)
}

func (s *syncer) syncReaderClusterRoleBindings(readerRoleName string, rr *radixv1.RadixRegistration) error {
	if appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups); len(appReaderSubjects) > 0 {
		readerClusterRoleBinding := internal.BuildClusterRoleBinding(readerRoleName, appReaderSubjects, s.radixDNSAlias)
		return s.kubeUtil.ApplyClusterRoleBinding(readerClusterRoleBinding)
	}
	return s.kubeUtil.DeleteClusterRoleBinding(readerRoleName)
}

func (s *syncer) syncClusterRoles(adminRoleName, readerRoleName string) error {
	adminClusterRole := s.buildDNSAliasClusterRole(adminRoleName, []string{"get", "list"})
	if err := s.kubeUtil.ApplyClusterRole(adminClusterRole); err != nil {
		return err
	}
	readerClusterRole := s.buildDNSAliasClusterRole(readerRoleName, []string{"get", "list"})
	return s.kubeUtil.ApplyClusterRole(readerClusterRole)
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
			OwnerReferences: internal.GetOwnerReferences(s.radixDNSAlias, false),
		},
		Rules: rules,
	}
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

func (s *syncer) getLabelsForDNSAliasRbac() labels.Set {
	return radixlabels.ForDNSAliasRbac(s.radixDNSAlias.Spec.AppName, s.radixDNSAlias.GetName())
}
