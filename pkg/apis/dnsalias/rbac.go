package dnsalias

import (
	"context"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/dnsalias/internal"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// syncRbac Grants access to Radix DNSAlias to application admin and reader
func (s *syncer) syncRbac(ctx context.Context) error {
	if err := s.syncAppAdminRbac(ctx); err != nil {
		return err
	}
	return s.syncAppReaderRbac(ctx)
}

func (s *syncer) syncAppAdminRbac(ctx context.Context) error {
	subjects := utils.GetAppAdminRbacSubjects(s.rr)
	roleName := s.getClusterRoleNameForAdmin()
	return s.syncClusterRoleAndBinding(ctx, roleName, subjects)
}

func (s *syncer) syncAppReaderRbac(ctx context.Context) error {
	subjects := utils.GetAppReaderRbacSubjects(s.rr)
	roleName := s.getClusterRoleNameForReader()
	return s.syncClusterRoleAndBinding(ctx, roleName, subjects)
}

func (s *syncer) syncClusterRoleAndBinding(ctx context.Context, roleName string, subjects []rbacv1.Subject) error {
	if err := s.syncClusterRole(ctx, roleName); err != nil {
		return err
	}

	return s.syncClusterRoleBinding(ctx, roleName, subjects)
}

func (s *syncer) syncClusterRole(ctx context.Context, roleName string) error {
	clusterRole, err := s.buildDNSAliasClusterRole(ctx, roleName, []string{"get", "list"})
	if err != nil {
		return err
	}
	// ownerReferences, err := s.getExistingClusterRoleOwnerReferences(ctx, roleName)
	// if err != nil {
	// 	return err
	// }
	// controllerutil.SetControllerReference()
	// 	controllerutil.CreateOrUpdate()
	// clusterRole.ObjectMeta.OwnerReferences = kube.MergeOwnerReferences(clusterRole.GetOwnerReferences(), ownerReferences...)
	return s.kubeUtil.ApplyClusterRole(ctx, clusterRole)
}

func (s *syncer) syncClusterRoleBinding(ctx context.Context, bindingName string, subjects []rbacv1.Subject) error {
	clusterRoleBinding, err := s.buildClusterRoleBindingsForSubjects(ctx, bindingName, subjects)
	if err != nil {
		return err
	}

	return s.kubeUtil.ApplyClusterRoleBinding(ctx, clusterRoleBinding)
}

func (s *syncer) buildDNSAliasClusterRole(ctx context.Context, clusterRoleName string, verbs []string) (*rbacv1.ClusterRole, error) {
	resourceNames, err := s.getClusterRoleResourceNames(ctx)
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

func (s *syncer) buildClusterRoleBindingsForSubjects(ctx context.Context, roleName string, subjects []rbacv1.Subject) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding, err := s.kubeUtil.KubeClient().RbacV1().ClusterRoleBindings().Get(ctx, roleName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		return internal.BuildClusterRoleBinding(roleName, subjects, s.radixDNSAlias), nil
	}

	clusterRoleBinding.Subjects = subjects
	// clusterRoleBinding.ObjectMeta.OwnerReferences = kube.MergeOwnerReferences(clusterRoleBinding.ObjectMeta.OwnerReferences, internal.GetOwnerReferences(s.radixDNSAlias, false)...)
	return clusterRoleBinding, nil
}

func (s *syncer) getClusterRoleNameForAdmin() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationAdminRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) getClusterRoleNameForReader() string {
	return fmt.Sprintf("%s-%s", defaults.RadixApplicationReaderRadixDNSAliasRoleNamePrefix, s.radixDNSAlias.Spec.AppName)
}

func (s *syncer) getClusterRoleResourceNames(ctx context.Context) ([]string, error) {
	dnsAliases, err := s.kubeUtil.ListRadixDNSAliasWithSelector(ctx, radixlabels.ForApplicationName(s.radixDNSAlias.Spec.AppName).String())
	if err != nil {
		return nil, err
	}
	return slice.Map(dnsAliases, func(dnsAlias *radixv1.RadixDNSAlias) string { return dnsAlias.GetName() }), nil
}

func (s *syncer) getLabelsForDNSAliasRbac() labels.Set {
	return radixlabels.ForDNSAliasRbac(s.radixDNSAlias.Spec.AppName)
}
