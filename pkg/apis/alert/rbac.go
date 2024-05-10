package alert

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (syncer *alertSyncer) configureRbac(ctx context.Context) error {
	rr, found := syncer.tryGetRadixRegistration(ctx)
	if !found {
		syncer.logger.Debug().Msg("radixregistration not found")
		return syncer.garbageCollectAccessToAlertConfigSecret(ctx)
	}

	if err := syncer.grantAdminAccessToAlertConfigSecret(ctx, rr); err != nil {
		return err
	}

	return syncer.grantReaderAccessToAlertConfigSecret(ctx, rr)
}

func (syncer *alertSyncer) tryGetRadixRegistration(ctx context.Context) (*radixv1.RadixRegistration, bool) {
	appName, found := syncer.radixAlert.Labels[kube.RadixAppLabel]
	if !found {
		return nil, false
	}

	rr, err := syncer.radixClient.RadixV1().RadixRegistrations().Get(ctx, appName, v1.GetOptions{})
	if err != nil {
		return nil, false
	}
	return rr, true
}

func (syncer *alertSyncer) garbageCollectAccessToAlertConfigSecret(ctx context.Context) error {
	namespace := syncer.radixAlert.Namespace

	for _, roleName := range []string{getAlertConfigSecretAdminRoleName(syncer.radixAlert.Name), getAlertConfigSecretReaderRoleName(syncer.radixAlert.Name)} {
		_, err := syncer.kubeUtil.GetRoleBinding(ctx, namespace, roleName)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			if err = syncer.kubeUtil.DeleteRoleBinding(ctx, namespace, roleName); err != nil {
				return err
			}
		}

		_, err = syncer.kubeUtil.GetRole(ctx, namespace, roleName)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			if err = syncer.kubeUtil.DeleteRole(ctx, namespace, roleName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (syncer *alertSyncer) grantAdminAccessToAlertConfigSecret(ctx context.Context, rr *radixv1.RadixRegistration) error {
	secretName := GetAlertSecretName(syncer.radixAlert.Name)
	roleName := getAlertConfigSecretAdminRoleName(syncer.radixAlert.Name)
	namespace := syncer.radixAlert.Namespace

	// create role
	role := kube.CreateManageSecretRole(rr.GetName(), roleName, []string{secretName}, nil)
	role.OwnerReferences = syncer.getOwnerReference()
	err := syncer.kubeUtil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	// create rolebinding
	adGroups, err := utils.GetAdGroups(rr)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	rolebinding.OwnerReferences = syncer.getOwnerReference()
	return syncer.kubeUtil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (syncer *alertSyncer) grantReaderAccessToAlertConfigSecret(ctx context.Context, rr *radixv1.RadixRegistration) error {
	secretName := GetAlertSecretName(syncer.radixAlert.Name)
	roleName := getAlertConfigSecretReaderRoleName(syncer.radixAlert.Name)
	namespace := syncer.radixAlert.Namespace

	// create role
	role := kube.CreateReadSecretRole(rr.GetName(), roleName, []string{secretName}, nil)
	role.OwnerReferences = syncer.getOwnerReference()
	err := syncer.kubeUtil.ApplyRole(ctx, namespace, role)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, role.Labels)
	rolebinding.OwnerReferences = syncer.getOwnerReference()
	return syncer.kubeUtil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func getAlertConfigSecretAdminRoleName(alertName string) string {
	return fmt.Sprintf("%s-admin", GetAlertSecretName(alertName))
}

func getAlertConfigSecretReaderRoleName(alertName string) string {
	return fmt.Sprintf("%s-reader", GetAlertSecretName(alertName))
}
