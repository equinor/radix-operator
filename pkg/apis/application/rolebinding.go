package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app *Application) applyRbacAppNamespace() error {
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
		err = k.ApplyRoleBinding(appNamespace, roleBinding)
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyRbacRadixRegistration Grants access to radix registration
func (app *Application) applyRbacRadixRegistration() error {
	rr := app.registration
	appName := rr.Name

	// Admin RBAC
	clusterRoleName := fmt.Sprintf("radix-platform-user-rr-%s", appName)
	adminClusterRole := app.buildRRClusterRole(clusterRoleName, []string{"get", "list", "watch", "update", "patch", "delete"})
	appAdminSubjects, err := utils.GetAppAdminRbacSubjects(rr)
	if err != nil {
		return err
	}
	adminClusterRoleBinding := app.rrClusterroleBinding(adminClusterRole, appAdminSubjects)

	// Reader RBAC
	clusterRoleReaderName := fmt.Sprintf("radix-platform-user-rr-reader-%s", appName)
	readerClusterRole := app.buildRRClusterRole(clusterRoleReaderName, []string{"get", "list", "watch"})
	appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	readerClusterRoleBinding := app.rrClusterroleBinding(readerClusterRole, appReaderSubjects)

	// Apply roles and bindings
	for _, clusterRole := range []*rbacv1.ClusterRole{adminClusterRole, readerClusterRole} {
		err := app.kubeutil.ApplyClusterRole(clusterRole)
		if err != nil {
			return err
		}
	}

	for _, clusterRoleBindings := range []*rbacv1.ClusterRoleBinding{adminClusterRoleBinding, readerClusterRoleBinding} {
		err := app.kubeutil.ApplyClusterRoleBinding(clusterRoleBindings)
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyRbacOnPipelineRunner Grants access to radix pipeline
func (app *Application) applyRbacOnPipelineRunner() error {
	serviceAccount, err := app.applyPipelineServiceAccount()
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
		return err
	}
	if err = app.givePipelineAccessToRR(serviceAccount, defaults.RadixPipelineRRRoleNamePrefix); err != nil {
		return err
	}
	if err = app.giveAccessToRadixDNSAliases(serviceAccount, defaults.RadixPipelineRadixDNSAliasRoleNamePrefix); err != nil {
		return err
	}
	return app.givePipelineAccessToAppNamespace(serviceAccount)
}

func (app *Application) applyRbacOnRadixTekton() error {
	serviceAccount, err := app.kubeutil.CreateServiceAccount(utils.GetAppNamespace(app.registration.Name), defaults.RadixTektonServiceAccountName)
	if err != nil {
		return err
	}
	if err = app.givePipelineAccessToRR(serviceAccount, defaults.RadixTektonRRRoleNamePrefix); err != nil {
		return err
	}
	if err = app.giveAccessToRadixDNSAliases(serviceAccount, defaults.RadixTektonRadixDNSAliasRoleNamePrefix); err != nil {
		return err
	}
	return app.giveRadixTektonAccessToAppNamespace(serviceAccount)
}

func (app *Application) givePipelineAccessToRR(serviceAccount *corev1.ServiceAccount, clusterRoleNamePrefix string) error {
	clusterRoleName := fmt.Sprintf("%s-%s", clusterRoleNamePrefix, app.registration.Name)
	clusterRole := app.buildRRClusterRole(clusterRoleName, []string{"get"})
	clusterRoleBinding := app.clusterRoleBinding(serviceAccount, clusterRole)
	return app.applyClusterRoleAndBinding(clusterRole, clusterRoleBinding)
}

func (app *Application) giveAccessToRadixDNSAliases(serviceAccount *corev1.ServiceAccount, clusterRoleNamePrefix string) error {
	clusterRole := app.buildRadixDNSAliasClusterRole(clusterRoleNamePrefix)
	clusterRoleBinding := app.clusterRoleBinding(serviceAccount, clusterRole)
	return app.applyClusterRoleAndBinding(clusterRole, clusterRoleBinding)
}

func (app *Application) applyClusterRoleAndBinding(clusterRole *rbacv1.ClusterRole, clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	if err := app.kubeutil.ApplyClusterRole(clusterRole); err != nil {
		return err
	}
	return app.kubeutil.ApplyClusterRoleBinding(clusterRoleBinding)
}

func (app *Application) givePipelineAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	rolebinding := app.pipelineRoleBinding(serviceAccount)
	return app.kubeutil.ApplyRoleBinding(namespace, rolebinding)
}

func (app *Application) giveRadixTektonAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	namespace := utils.GetAppNamespace(app.registration.Name)
	roleBinding := app.radixTektonRoleBinding(serviceAccount)
	return app.kubeutil.ApplyRoleBinding(namespace, roleBinding)
}

func (app *Application) pipelineRoleBinding(serviceAccount *corev1.ServiceAccount) *rbacv1.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.PipelineAppRoleName)

	rolebinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       "RoleBinding",
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
	logger.Debugf("Create rolebinding config %s", defaults.RadixTektonAppRoleName)

	rolebinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       "RoleBinding",
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
	logger.Debugf("Create clusterrolebinding config %s", clusterRoleBindingName)

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

func (app *Application) rrClusterroleBinding(clusterrole *rbacv1.ClusterRole, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)
	ownerReference := app.getOwnerReference()

	clusterrolebinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.Identifier(),
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     k8s.KindClusterRole,
			Name:     clusterrole.Name,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create clusterrolebinding config %s", clusterroleBindingName)

	return clusterrolebinding
}
