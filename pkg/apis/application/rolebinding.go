package application

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *Application) applyRbacAppNamespace(ctx context.Context) error {
	k := app.kubeutil
	registration := app.registration

	appNamespace := utils.GetAppNamespace(registration.Name)
	adGroups, err := utils.GetAdGroups(registration)
	if err != nil {
		return err
	}
	subjects := kube.GetRoleBindingGroups(adGroups)
	adminRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(registration.Name, defaults.AppAdminRoleName, subjects)

	readerAdGroups := registration.Spec.ReaderAdGroups
	readerSubjects := kube.GetRoleBindingGroups(readerAdGroups)
	readerRoleBinding := kube.GetRolebindingToClusterRoleForSubjects(registration.Name, defaults.AppReaderRoleName, readerSubjects)

	for _, roleBinding := range []*rbacv1.RoleBinding{adminRoleBinding, readerRoleBinding} {
		err = k.ApplyRoleBinding(ctx, appNamespace, roleBinding)
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyRbacRadixRegistration Grants access to radix registration
func (app *Application) applyRbacRadixRegistration(ctx context.Context) error {
	rr := app.registration
	appName := rr.Name

	// Admin RBAC
	clusterRoleName := fmt.Sprintf("radix-platform-user-rr-%s", appName)
	adminClusterRole := app.buildRRClusterRole(clusterRoleName, []string{"get", "list", "watch", "update", "patch", "delete"})
	appAdminSubjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}
	adminClusterRoleBinding := app.rrClusterRoleBinding(adminClusterRole, appAdminSubjects)

	// Reader RBAC
	clusterRoleReaderName := fmt.Sprintf("radix-platform-user-rr-reader-%s", appName)
	readerClusterRole := app.buildRRClusterRole(clusterRoleReaderName, []string{"get", "list", "watch"})
	appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	readerClusterRoleBinding := app.rrClusterRoleBinding(readerClusterRole, appReaderSubjects)

	// Apply roles and bindings
	for _, clusterRole := range []*rbacv1.ClusterRole{adminClusterRole, readerClusterRole} {
		err := app.kubeutil.ApplyClusterRole(ctx, clusterRole)
		if err != nil {
			return err
		}
	}

	for _, clusterRoleBindings := range []*rbacv1.ClusterRoleBinding{adminClusterRoleBinding, readerClusterRoleBinding} {
		err := app.kubeutil.ApplyClusterRoleBinding(ctx, clusterRoleBindings)
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyRbacOnPipelineRunner Grants access to radix pipeline
func (app *Application) applyRbacOnPipelineRunner(ctx context.Context) error {
	serviceAccount, err := app.applyPipelineServiceAccount(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply pipeline service account: %w", err)
	}

	if err = app.givePipelineAccessToRR(ctx, serviceAccount, defaults.RadixPipelineRRRoleNamePrefix); err != nil {
		return fmt.Errorf("failed to grant pipeline access to RadixRegistration: %w", err)
	}

	if err = app.giveAccessToRadixDNSAliases(ctx, serviceAccount, defaults.RadixPipelineRadixDNSAliasRoleNamePrefix); err != nil {
		return fmt.Errorf("failed to grant pipeline access to RadixDNSAliases: %w", err)
	}

	if err := app.givePipelineAccessToAppNamespace(ctx, serviceAccount); err != nil {
		return fmt.Errorf("failed to grant pipeline access to app namespace: %w", err)
	}

	return nil
}

func (app *Application) applyRbacOnRadixTekton(ctx context.Context) error {
	serviceAccount, err := app.kubeutil.CreateServiceAccount(ctx, utils.GetAppNamespace(app.registration.Name), defaults.RadixTektonServiceAccountName)
	if err != nil {
		return fmt.Errorf("failed to apply Tekton pipeline service account: %w", err)
	}

	if err = app.givePipelineAccessToRR(ctx, serviceAccount, defaults.RadixTektonRRRoleNamePrefix); err != nil {
		return fmt.Errorf("failed to grant Tekton pipeline access to RadixRegistration: %w", err)
	}

	if err = app.giveAccessToRadixDNSAliases(ctx, serviceAccount, defaults.RadixTektonRadixDNSAliasRoleNamePrefix); err != nil {
		return fmt.Errorf("failed to grant Tekton pipeline access to RadixDNSAliases: %w", err)
	}

	if err := app.giveRadixTektonAccessToAppNamespace(ctx, serviceAccount); err != nil {
		return fmt.Errorf("failed to grant Tekton pipeline access to app namespace: %w", err)
	}

	return nil
}

func (app *Application) givePipelineAccessToRR(ctx context.Context, serviceAccount *corev1.ServiceAccount, clusterRoleNamePrefix string) error {
	clusterRoleName := fmt.Sprintf("%s-%s", clusterRoleNamePrefix, app.registration.Name)
	clusterRole := app.buildRRClusterRole(clusterRoleName, []string{"get"})
	clusterRoleBinding := app.clusterRoleBinding(serviceAccount, clusterRole)
	return app.applyClusterRoleAndBinding(ctx, clusterRole, clusterRoleBinding)
}

func (app *Application) giveAccessToRadixDNSAliases(ctx context.Context, serviceAccount *corev1.ServiceAccount, clusterRoleNamePrefix string) error {
	clusterRole := app.buildRadixDNSAliasClusterRole(clusterRoleNamePrefix)
	clusterRoleBinding := app.clusterRoleBinding(serviceAccount, clusterRole)
	return app.applyClusterRoleAndBinding(ctx, clusterRole, clusterRoleBinding)
}

func (app *Application) applyClusterRoleAndBinding(ctx context.Context, clusterRole *rbacv1.ClusterRole, clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	if err := app.kubeutil.ApplyClusterRole(ctx, clusterRole); err != nil {
		return err
	}
	return app.kubeutil.ApplyClusterRoleBinding(ctx, clusterRoleBinding)
}

func (app *Application) givePipelineAccessToAppNamespace(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	rolebinding := app.pipelineRoleBinding(serviceAccount)
	return app.kubeutil.ApplyRoleBinding(ctx, namespace, rolebinding)
}

func (app *Application) giveRadixTektonAccessToAppNamespace(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	roleBinding := app.radixTektonRoleBinding(serviceAccount)
	return app.kubeutil.ApplyRoleBinding(ctx, namespace, roleBinding)
}

func (app *Application) pipelineRoleBinding(serviceAccount *corev1.ServiceAccount) *rbacv1.RoleBinding {
	registration := app.registration
	appName := registration.Name
	app.logger.Debug().Msgf("Create rolebinding config %s", defaults.PipelineAppRoleName)

	rolebinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.PipelineAppRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.PipelineAppRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app *Application) radixTektonRoleBinding(serviceAccount *corev1.ServiceAccount) *rbacv1.RoleBinding {
	registration := app.registration
	appName := registration.Name
	app.logger.Debug().Msgf("Create rolebinding config %s", defaults.RadixTektonAppRoleName)

	rolebinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.RadixTektonAppRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.RadixTektonAppRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app *Application) clusterRoleBinding(serviceAccount *corev1.ServiceAccount, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRoleBinding {
	appName := app.registration.Name
	clusterRoleBindingName := clusterRole.Name
	ownerReference := app.getOwnerReference()
	app.logger.Debug().Msgf("Create clusterrolebinding config %s", clusterRoleBindingName)

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
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return clusterRoleBinding
}

func (app *Application) rrClusterRoleBinding(clusterRole *rbacv1.ClusterRole, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterRoleBindingName := clusterRole.Name
	app.logger.Debug().Msgf("Create clusterrolebinding config %s", clusterRoleBindingName)
	ownerReference := app.getOwnerReference()

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

	app.logger.Debug().Msgf("Done - create clusterrolebinding config %s", clusterRoleBindingName)

	return clusterRoleBinding
}
