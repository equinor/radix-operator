package dnsalias

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// syncRbac Grants access to Radix DNSAlias to application admin and reader
func (s *syncer) syncRbac() error {
	rr, err := s.kubeUtil.GetRegistration(s.radixDNSAlias.Spec.AppName)
	if err != nil {
		return err
	}
	if err := s.syncAppAdminRbac(rr); err != nil {
		return err
	}
	return s.syncAppReaderRbac(rr)
}

func (s *syncer) syncAppAdminRbac(rr *radixv1.RadixRegistration) error {
	subjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}
	roleName := s.getClusterRoleNameForAdmin()
	return s.syncClusterRoleAndBinding(roleName, subjects)
}

func (s *syncer) syncAppReaderRbac(rr *radixv1.RadixRegistration) error {
	subjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	roleName := s.getClusterRoleNameForReader()
	return s.syncClusterRoleAndBinding(roleName, subjects)
}

func (s *syncer) syncClusterRoleAndBinding(roleName string, subjects []rbacv1.Subject) error {
	clusterRoleBinding, err := s.buildClusterRoleBindingsForSubjects(roleName, subjects)
	if err != nil {
		return err
	}
	if len(clusterRoleBinding.Subjects) == 0 {
		return s.deleteClusterRoleAndBinding(roleName)
	}
	if err := s.syncClusterRole(roleName); err != nil {
		return err
	}
	return s.kubeUtil.ApplyClusterRoleBinding(clusterRoleBinding)
}

func (s *syncer) buildClusterRoleBindingsForSubjects(roleName string, subjects []rbacv1.Subject) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding, err := s.kubeUtil.KubeClient().RbacV1().ClusterRoleBindings().Get(context.Background(), roleName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		return internal.BuildClusterRoleBinding(roleName, subjects, s.radixDNSAlias), nil
	}
	clusterRoleBinding.Subjects = subjects
	clusterRoleBinding.ObjectMeta.OwnerReferences = kube.MergeOwnerReferences(clusterRoleBinding.ObjectMeta.OwnerReferences, internal.GetOwnerReferences(s.radixDNSAlias, false)...)
	return clusterRoleBinding, nil
}

func (s *syncer) syncClusterRole(roleName string) error {
	clusterRole, err := s.buildDNSAliasClusterRole(roleName, []string{"get", "list"})
	if err != nil {
		return err
	}
	ownerReferences, err := s.getExistingClusterRoleOwnerReferences(roleName)
	if err != nil {
		return err
	}
	clusterRole.ObjectMeta.OwnerReferences = kube.MergeOwnerReferences(clusterRole.GetOwnerReferences(), ownerReferences...)
	return s.kubeUtil.ApplyClusterRole(clusterRole)
}

func (s *syncer) getClusterRoleNameForAdmin() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationAdminRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) getClusterRoleNameForReader() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationReaderRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) buildDNSAliasClusterRole(clusterRoleName string, verbs []string) (*rbacv1.ClusterRole, error) {
	resourceNames, err := s.getClusterRoleResourceNames()
	if err != nil {
		return nil, err
	}
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
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{radixv1.SchemeGroupVersion.Group},
			Resources:     []string{radixv1.ResourceRadixDNSAliases, radixv1.ResourceRadixDNSAliasStatuses},
			ResourceNames: resourceNames,
			Verbs:         verbs,
		}},
	}, nil
}

func (s *syncer) getClusterRoleResourceNames() ([]string, error) {
	dnsAliases, err := s.kubeUtil.ListRadixDNSAliasWithSelector(radixlabels.ForApplicationName(s.radixDNSAlias.Spec.AppName).String())
	if err != nil {
		return nil, err
	}
	return slice.Reduce(dnsAliases, []string{}, func(acc []string, dnsAlias *radixv1.RadixDNSAlias) []string {
		acc = append(acc, dnsAlias.GetName())
		return acc
	}), nil
}

func (s *syncer) deleteRbac() error {
	if err := s.deleteRbacByName(s.getClusterRoleNameForAdmin()); err != nil {
		return err
	}
	return s.deleteRbacByName(s.getClusterRoleNameForReader())
}

func (s *syncer) deleteRbacByName(roleName string) error {
	clusterRole, err := s.kubeUtil.GetClusterRole(roleName)
	if err != nil {
		return err
	}
	if len(clusterRole.Rules) == 0 {
		return s.deleteClusterRoleAndBinding(roleName)
	}
	dnsAliasName := s.radixDNSAlias.GetName()
	clusterRole.Rules[0].ResourceNames = getRoleResourceNamesWithout(dnsAliasName, clusterRole)
	if len(clusterRole.Rules[0].ResourceNames) == 0 {
		return s.deleteClusterRoleAndBinding(roleName)
	}

	clusterRole.ObjectMeta.OwnerReferences = slices.DeleteFunc(clusterRole.ObjectMeta.OwnerReferences,
		func(o metav1.OwnerReference) bool { return o.Name == dnsAliasName })
	return s.kubeUtil.ApplyClusterRole(clusterRole)
}

func getRoleResourceNamesWithout(dnsAliasName string, role *rbacv1.ClusterRole) []string {
	return slice.Reduce(role.Rules[0].ResourceNames, []string{}, func(acc []string, resourceName string) []string {
		if resourceName != dnsAliasName {
			acc = append(acc, resourceName)
		}
		return acc
	})
}

func (s *syncer) deleteClusterRoleAndBinding(roleName string) error {
	if err := s.kubeUtil.DeleteClusterRoleBinding(roleName); err != nil {
		return err
	}
	return s.kubeUtil.DeleteClusterRole(roleName)
}

func (s *syncer) getLabelsForDNSAliasRbac() labels.Set {
	return radixlabels.ForDNSAliasRbac(s.radixDNSAlias.Spec.AppName)
}
