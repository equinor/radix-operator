package application

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrantAccessToCICDLogs Grants access to pipeline logs
func (app Application) grantAccessToCICDLogs() error {
	k := app.kubeutil
	registration := app.registration

	namespace := utils.GetAppNamespace(registration.Name)

	adGroups, err := GetAdGroups(registration)
	if err != nil {
		return err
	}

	roleBinding := kube.GetRolebindingToClusterRole(registration.Name, defaults.AppAdminRoleName, adGroups)
	return k.ApplyRoleBinding(namespace, roleBinding)
}

// ApplyRbacRadixRegistration Grants access to radix registration
func (app Application) applyRbacRadixRegistration() error {
	k := app.kubeutil

	clusterrole := app.rrUserClusterRole()
	clusterrolebinding := app.rrClusterroleBinding(clusterrole)

	err := k.ApplyClusterRole(clusterrole)
	if err != nil {
		return err
	}

	return k.ApplyClusterRoleBinding(clusterrolebinding)
}

// ApplyRbacOnPipelineRunner Grants access to radix pipeline
func (app Application) applyRbacOnPipelineRunner(serviceAccount *corev1.ServiceAccount) error {
	err := app.givePipelineAccessToRR(serviceAccount)
	if err != nil {
		return err
	}

	err = app.givePipelineAccessToAppNamespace(serviceAccount)
	if err != nil {
		return err
	}

	return app.givePipelineAccessToDefaultNamespace(serviceAccount)
}

func (app Application) applyRbacOnConfigToMapRunner() error {
	serviceAccount, err := app.applyConfigToMapServiceAccount()
	if err != nil {
		return err
	}

	return app.giveConfigToMapRunnerAccessToAppNamespace(serviceAccount)
}

func (app Application) givePipelineAccessToRR(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil

	clusterrole := app.rrPipelineClusterRole()
	clusterrolebinding := app.rrPipelineClusterRoleBinding(serviceAccount, clusterrole)

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

func (app Application) giveConfigToMapRunnerAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil
	registration := app.registration

	namespace := utils.GetAppNamespace(registration.Name)

	// create role
	role := app.configToMapRunnerRole()
	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// Create role binding
	rolebinding := app.configToMapRunnerRoleBinding(serviceAccount)
	return k.ApplyRoleBinding(namespace, rolebinding)
}

func (app Application) givePipelineAccessToDefaultNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil

	rolebinding := app.pipelineClusterRolebinding(serviceAccount)

	return k.ApplyClusterRoleBinding(rolebinding)
}

func (app Application) pipelineClusterRolebinding(serviceAccount *corev1.ServiceAccount) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	ownerReference := app.getOwnerReference()
	logger.Debugf("Create cluster rolebinding config %s", defaults.PipelineRunnerRoleName)

	rolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", defaults.PipelineRunnerRoleName, appName),
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.PipelineRunnerRoleName,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app Application) pipelineRoleBinding(serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.PipelineRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.PipelineRoleName,
			Labels: map[string]string{
				"radixReg": appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.PipelineRoleName,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app Application) configToMapRunnerRoleBinding(serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.ConfigToMapRunnerRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.ConfigToMapRunnerRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     defaults.ConfigToMapRunnerRoleName,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return rolebinding
}

func (app Application) rrPipelineClusterRoleBinding(serviceAccount *corev1.ServiceAccount, clusterrole *auth.ClusterRole) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	ownerReference := app.getOwnerReference()
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterrole.Name,
		},
		Subjects: []auth.Subject{
			auth.Subject{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	return clusterrolebinding
}

func (app Application) rrClusterroleBinding(clusterrole *auth.ClusterRole) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)

	ownerReference := app.getOwnerReference()

	adGroups, _ := GetAdGroups(registration)
	subjects := kube.GetRoleBindingGroups(adGroups)

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterrole.Name,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create clusterrolebinding config %s", clusterroleBindingName)

	return clusterrolebinding
}
