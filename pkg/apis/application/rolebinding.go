package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (app Application) applyRbacAppNamespace() error {
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

	for _, roleBinding := range []*auth.RoleBinding{adminRoleBinding, readerRoleBinding} {
		err = k.ApplyRoleBinding(appNamespace, roleBinding)
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyRbacRadixRegistration Grants access to radix registration
func (app Application) applyRbacRadixRegistration() error {
	rr := app.registration
	appName := rr.Name

	// Admin RBAC
	clusterRoleName := fmt.Sprintf("radix-platform-user-rr-%s", appName)
	adminClusterRole := app.rrClusterRole(clusterRoleName, []string{"get", "list", "watch", "update", "patch", "delete"})
	appAdminSubjects, err := getAppAdminSubjects(rr)
	if err != nil {
		return err
	}
	adminClusterRoleBinding := app.rrClusterroleBinding(adminClusterRole, appAdminSubjects)

	// Reader RBAC
	clusterRoleReaderName := fmt.Sprintf("radix-platform-user-rr-reader-%s", appName)
	readerClusterRole := app.rrClusterRole(clusterRoleReaderName, []string{"get", "list", "watch"})
	appReaderSubjects := kube.GetRoleBindingGroups(rr.Spec.ReaderAdGroups)
	readerClusterRoleBinding := app.rrClusterroleBinding(readerClusterRole, appReaderSubjects)

	// Apply roles and bindings
	for _, clusterRole := range []*auth.ClusterRole{adminClusterRole, readerClusterRole} {
		err := app.kubeutil.ApplyClusterRole(clusterRole)
		if err != nil {
			return err
		}
	}

	for _, clusterRoleBindings := range []*auth.ClusterRoleBinding{adminClusterRoleBinding, readerClusterRoleBinding} {
		err := app.kubeutil.ApplyClusterRoleBinding(clusterRoleBindings)
		if err != nil {
			return err
		}
	}

	return nil
}

func getAppAdminSubjects(rr *v1.RadixRegistration) ([]auth.Subject, error) {
	adGroups, err := utils.GetAdGroups(rr)
	if err != nil {
		return nil, err
	}
	subjects := kube.GetRoleBindingGroups(adGroups)
	return subjects, nil
}

// ApplyRbacOnPipelineRunner Grants access to radix pipeline
func (app Application) applyRbacOnPipelineRunner() error {
	serviceAccount, err := app.applyPipelineServiceAccount()
	if err != nil {
		logger.Errorf("Failed to apply service account needed by pipeline. %v", err)
		return err
	}

	err = app.givePipelineAccessToRR(serviceAccount, "radix-pipeline-rr")
	if err != nil {
		return err
	}

	return app.givePipelineAccessToAppNamespace(serviceAccount)
}

func (app Application) applyRbacOnRadixTekton() error {
	serviceAccount, err := app.kubeutil.CreateServiceAccount(utils.GetAppNamespace(app.registration.Name), defaults.RadixTektonServiceAccountName)
	if err != nil {
		return err
	}

	err = app.givePipelineAccessToRR(serviceAccount, "radix-tekton-rr")
	if err != nil {
		return err
	}

	return app.giveRadixTektonAccessToAppNamespace(serviceAccount)
}

func (app Application) givePipelineAccessToRR(serviceAccount *corev1.ServiceAccount, clusterRoleNamePrefix string) error {
	k := app.kubeutil

	clusterrole := app.rrPipelineClusterRole(clusterRoleNamePrefix)
	clusterrolebinding := app.rrClusterRoleBinding(serviceAccount, clusterrole)

	err := k.ApplyClusterRole(clusterrole)
	if err != nil {
		return err
	}

	err = k.ApplyClusterRoleBinding(clusterrolebinding)
	if err != nil {
		return err
	}
	return nil
}

func (app Application) givePipelineAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil
	registration := app.registration

	namespace := utils.GetAppNamespace(registration.Name)
	rolebinding := app.pipelineRoleBinding(serviceAccount)

	return k.ApplyRoleBinding(namespace, rolebinding)
}

func (app Application) giveRadixTektonAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil
	registration := app.registration

	namespace := utils.GetAppNamespace(registration.Name)

	// Create role binding
	roleBinding := app.radixTektonRoleBinding(serviceAccount)
	return k.ApplyRoleBinding(namespace, roleBinding)
}

func (app Application) pipelineRoleBinding(serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.PipelineAppRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.PipelineAppRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.PipelineAppRoleName,
		},
		Subjects: []auth.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app Application) radixTektonRoleBinding(serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.RadixTektonAppRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.RadixTektonAppRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     defaults.RadixTektonAppRoleName,
		},
		Subjects: []auth.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app Application) rrClusterRoleBinding(serviceAccount *corev1.ServiceAccount, clusterrole *auth.ClusterRole) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	ownerReference := app.getOwnerReference()
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     clusterrole.Name,
		},
		Subjects: []auth.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return clusterrolebinding
}

func (app Application) rrClusterroleBinding(clusterrole *auth.ClusterRole, subjects []auth.Subject) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)
	ownerReference := app.getOwnerReference()

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8s.RbacApiVersion,
			Kind:       k8s.KindClusterRoleBinding,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: k8s.RbacApiGroup,
			Kind:     k8s.KindClusterRole,
			Name:     clusterrole.Name,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create clusterrolebinding config %s", clusterroleBindingName)

	return clusterrolebinding
}
