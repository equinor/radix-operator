package alert

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (syncer *alertSyncer) configureRbac(ctx context.Context) error {
	rr, found := syncer.tryGetRadixRegistration(ctx)
	if !found {
		log.Ctx(ctx).Debug().Msg("radixregistration not found")
		return syncer.garbageCollectAccessToAlertConfigSecret(ctx)
	}

	if err := syncer.reconcileAdminAccessToAlertConfigSecret(ctx, rr); err != nil {
		return err
	}

	return syncer.reconcileReaderAccessToAlertConfigSecret(ctx, rr)
}

func (syncer *alertSyncer) tryGetRadixRegistration(ctx context.Context) (*radixv1.RadixRegistration, bool) {
	appName, found := syncer.radixAlert.Labels[kube.RadixAppLabel]
	if !found {
		return nil, false
	}

	rr := &radixv1.RadixRegistration{}
	if err := syncer.dynamicClient.Get(ctx, client.ObjectKey{Name: appName}, rr); err != nil {
		return nil, false
	}
	return rr, true
}

func (syncer *alertSyncer) garbageCollectAccessToAlertConfigSecret(ctx context.Context) error {
	namespace := syncer.radixAlert.Namespace

	for _, roleName := range []string{getAlertConfigSecretAdminRoleName(syncer.radixAlert.Name), getAlertConfigSecretReaderRoleName(syncer.radixAlert.Name)} {
		rolebinding := &rbacv1.RoleBinding{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
		if err := syncer.dynamicClient.Delete(ctx, rolebinding); client.IgnoreNotFound(err) != nil {
			return err
		}

		role := &rbacv1.Role{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
		if err := syncer.dynamicClient.Delete(ctx, role); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func (syncer *alertSyncer) reconcileAdminAccessToAlertConfigSecret(ctx context.Context, rr *radixv1.RadixRegistration) error {
	namespace := syncer.radixAlert.Namespace
	roleName := getAlertConfigSecretAdminRoleName(syncer.radixAlert.Name)

	// create role
	r := rbacv1.Role{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, &r, func() error {
		secretName := GetAlertSecretName(syncer.radixAlert.Name)
		role := kube.CreateManageSecretRole(rr.GetName(), roleName, []string{secretName}, nil)
		r.Rules = role.Rules
		r.Labels = role.Labels

		return controllerutil.SetControllerReference(syncer.radixAlert, &r, syncer.dynamicClient.Scheme())
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("op", string(op)).Msg("reconcile role for admin access to AlertConfigSecret")
	}

	// create rolebinding
	rb := &rbacv1.RoleBinding{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
	op, err = controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, rb, func() error {
		subjects := utils.GetAppAdminRbacSubjects(rr)
		rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, r.Labels)
		rb.RoleRef = rolebinding.RoleRef
		rb.Subjects = rolebinding.Subjects
		rb.Labels = rolebinding.Labels

		return controllerutil.SetControllerReference(syncer.radixAlert, rb, syncer.dynamicClient.Scheme())
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("op", string(op)).Msg("reconcile rolebinding for admin access to AlertConfigSecret")
	}

	return nil
}

func (syncer *alertSyncer) reconcileReaderAccessToAlertConfigSecret(ctx context.Context, rr *radixv1.RadixRegistration) error {
	roleName := getAlertConfigSecretReaderRoleName(syncer.radixAlert.Name)
	namespace := syncer.radixAlert.Namespace

	// create role
	r := rbacv1.Role{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, &r, func() error {
		secretName := GetAlertSecretName(syncer.radixAlert.Name)
		role := kube.CreateReadSecretRole(rr.GetName(), roleName, []string{secretName}, nil)
		r.Rules = role.Rules
		r.Labels = role.Labels

		return controllerutil.SetControllerReference(syncer.radixAlert, &r, syncer.dynamicClient.Scheme())
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("op", string(op)).Msg("reconcile role for reader access to AlertConfigSecret")
	}

	// create rolebinding
	rb := &rbacv1.RoleBinding{ObjectMeta: v1.ObjectMeta{Name: roleName, Namespace: namespace}}
	op, err = controllerutil.CreateOrUpdate(ctx, syncer.dynamicClient, rb, func() error {
		subjects := utils.GetAppReaderRbacSubjects(rr)
		rolebinding := kube.GetRolebindingToRoleWithLabelsForSubjects(roleName, subjects, r.Labels)
		rb.RoleRef = rolebinding.RoleRef
		rb.Subjects = rolebinding.Subjects
		rb.Labels = rolebinding.Labels

		return controllerutil.SetControllerReference(syncer.radixAlert, rb, syncer.dynamicClient.Scheme())
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		log.Ctx(ctx).Info().Str("op", string(op)).Msg("reconcile rolebinding for reader access to AlertConfigSecret")
	}
	return nil
}

func getAlertConfigSecretAdminRoleName(alertName string) string {
	return fmt.Sprintf("%s-admin", GetAlertSecretName(alertName))
}

func getAlertConfigSecretReaderRoleName(alertName string) string {
	return fmt.Sprintf("%s-reader", GetAlertSecretName(alertName))
}
