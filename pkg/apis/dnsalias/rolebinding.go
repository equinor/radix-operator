package dnsalias

import (
	"fmt"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	logger "github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// syncRbac Grants access to Radix DNSAlias to application admin and reader
func (s *syncer) syncRbac() error {
	appName := s.radixDNSAlias.Spec.AppName
	rr, err := s.kubeUtil.GetRegistration(appName)
	if err != nil {
		return err
	}
	// Admin RBAC
	clusterRoleName := fmt.Sprintf("%s-%s", defaults.RadixApplicationAdminRadixDNSAliasRoleNamePrefix, appName)
	adminClusterRole := s.buildDNSAliasClusterRole(clusterRoleName, []string{"get", "list"})
	appAdminSubjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}
	adminClusterRoleBinding := s.buildDNSAliasClusterRoleBinding(appName, adminClusterRole, appAdminSubjects)

	// Reader RBAC
	clusterRoleReaderName := fmt.Sprintf("%s-%s", defaults.RadixApplicationReaderRadixDNSAliasRoleNamePrefix, appName)
	readerClusterRole := s.buildDNSAliasClusterRole(clusterRoleReaderName, []string{"get", "list"})
	appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	readerClusterRoleBinding := s.buildDNSAliasClusterRoleBinding(appName, readerClusterRole, appReaderSubjects)

	// Apply roles and bindings
	for _, clusterRole := range []*rbacv1.ClusterRole{adminClusterRole, readerClusterRole} {
		if err := s.kubeUtil.ApplyClusterRole(clusterRole); err != nil {
			return err
		}
	}

	for _, clusterRoleBindings := range []*rbacv1.ClusterRoleBinding{adminClusterRoleBinding, readerClusterRoleBinding} {
		if err := s.kubeUtil.ApplyClusterRoleBinding(clusterRoleBindings); err != nil {
			return err
		}
	}

	return nil
}

func (s *syncer) buildDNSAliasClusterRole(clusterRoleName string, verbs []string) *rbacv1.ClusterRole {
	return s.buildClusterRole(clusterRoleName, rbacv1.PolicyRule{APIGroups: []string{radixv1.SchemeGroupVersion.Group},
		Resources:     []string{radixv1.ResourceRadixDNSAliases},
		ResourceNames: []string{s.radixDNSAlias.GetName()},
		Verbs:         verbs,
	})
}

func (s *syncer) buildClusterRole(clusterRoleName string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	logger.Debugf("Creating clusterrole config %s", clusterRoleName)
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRole,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: s.radixDNSAlias.Spec.AppName,
			},
			OwnerReferences: s.getOwnerReference(),
		},
		Rules: rules,
	}
	logger.Debugf("Done - creating clusterrole config %s", clusterRoleName)
	return clusterRole
}

func (s *syncer) buildDNSAliasClusterRoleBinding(appName string, clusterRole *rbacv1.ClusterRole, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	clusterRoleBindingName := clusterRole.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterRoleBindingName)
	ownerReference := s.getOwnerReference()

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     clusterRole.Name,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create clusterrolebinding config %s", clusterRoleBindingName)

	return clusterRoleBinding
}

func (s *syncer) getOwnerReference() []metav1.OwnerReference {
	return []metav1.OwnerReference{{APIVersion: radixv1.SchemeGroupVersion.Identifier(), Kind: radixv1.KindRadixDNSAlias, Name: s.radixDNSAlias.GetName(), UID: s.radixDNSAlias.GetUID(), Controller: pointers.Ptr(true)}}
}
