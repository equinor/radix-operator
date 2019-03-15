package application

import (
	"fmt"

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
	subjects := kube.GetRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":         registration.Name, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel: registration.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: subjects,
	}

	return k.ApplyRoleBinding(namespace, roleBinding)
}

// ApplyRbacRadixRegistration Grants access to radix registration
func (app Application) applyRbacRadixRegistration() error {
	namespace := "default"
	k := app.kubeutil

	role := app.rrUserRole()
	rolebinding := app.rrRoleBinding(role)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	return k.ApplyRoleBinding(namespace, rolebinding)
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

func (app Application) givePipelineAccessToRR(serviceAccount *corev1.ServiceAccount) error {
	namespace := "default"
	k := app.kubeutil

	role := app.rrPipelineRole()
	rolebinding := app.rrPipelineRoleBinding(serviceAccount, role)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	err = k.ApplyRoleBinding(namespace, rolebinding)
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

func (app Application) givePipelineAccessToDefaultNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil

	rolebinding := app.pipelineClusterRolebinding(serviceAccount)

	return k.ApplyClusterRoleBinding(rolebinding)
}

func (app Application) pipelineClusterRolebinding(serviceAccount *corev1.ServiceAccount) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	roleName := "radix-pipeline-runner"
	ownerReference := app.getOwnerReference()
	logger.Debugf("Create cluster rolebinding config %s", roleName)

	rolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", roleName, appName),
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
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
	roleName := "radix-pipeline"
	logger.Debugf("Create rolebinding config %s", roleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixReg": appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
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

func (app Application) rrPipelineRoleBinding(serviceAccount *corev1.ServiceAccount, role *auth.Role) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	roleBindingName := role.Name
	ownerReference := app.getOwnerReference()
	logger.Debugf("Create rolebinding config %s", roleBindingName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
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

func (app Application) rrRoleBinding(role *auth.Role) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	roleBindingName := role.Name
	logger.Debugf("Create roleBinding config %s", roleBindingName)

	ownerReference := app.getOwnerReference()
	subjects := kube.GetRoleBindingGroups(registration.Spec.AdGroups)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixReg": appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create rolebinding config %s", roleBindingName)

	return rolebinding
}
