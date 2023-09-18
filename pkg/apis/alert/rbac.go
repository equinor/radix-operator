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

func (syncer *alertSyncer) configureRbac() error {
	rr, found := syncer.tryGetRadixRegistration()
	if !found {
		syncer.logger.Debug("radixregistration not found")
		return syncer.garbageCollectAccessToAlertConfigSecret()
	}

	return syncer.grantAccessToAlertConfigSecret(rr)
}

func (syncer *alertSyncer) tryGetRadixRegistration() (*radixv1.RadixRegistration, bool) {
	appName, found := syncer.radixAlert.Labels[kube.RadixAppLabel]
	if !found {
		return nil, false
	}

	rr, err := syncer.radixClient.RadixV1().RadixRegistrations().Get(context.TODO(), appName, v1.GetOptions{})
	if err != nil {
		return nil, false
	}
	return rr, true
}

func (syncer *alertSyncer) garbageCollectAccessToAlertConfigSecret() error {
	roleName := getAlertConfigSecretRoleName(syncer.radixAlert.Name)
	namespace := syncer.radixAlert.Namespace

	_, err := syncer.kubeUtil.GetRoleBinding(namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if err = syncer.kubeUtil.DeleteRoleBinding(namespace, roleName); err != nil {
			return err
		}
	}

	_, err = syncer.kubeUtil.GetRole(namespace, roleName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if err = syncer.kubeUtil.DeleteRole(namespace, roleName); err != nil {
			return err
		}
	}

	return nil
}

func (syncer *alertSyncer) grantAccessToAlertConfigSecret(rr *radixv1.RadixRegistration) error {
	secretName := GetAlertSecretName(syncer.radixAlert.Name)
	roleName := getAlertConfigSecretRoleName(syncer.radixAlert.Name)
	namespace := syncer.radixAlert.Namespace

	// create role
	role := kube.CreateManageSecretRole(rr.GetName(), roleName, []string{secretName}, nil)
	role.OwnerReferences = syncer.getOwnerReference()
	err := syncer.kubeUtil.ApplyRole(namespace, role)
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
	return syncer.kubeUtil.ApplyRoleBinding(namespace, rolebinding)
}

func getAlertConfigSecretRoleName(alertName string) string {
	return fmt.Sprintf("%s-admin", GetAlertSecretName(alertName))
}
