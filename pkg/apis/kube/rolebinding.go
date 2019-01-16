package kube

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	corev1 "k8s.io/api/core/v1"
	auth "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrantAppAdminAccessToNs Grant access to environment namespace
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) GrantAppAdminAccessToNs(namespace string, registration *radixv1.RadixRegistration) error {
	subjects := getRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin-envs"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: registration.Name,
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

// GrantAppAdminAccessToRuntimeSecrets Grants access to runtime secrets in environment namespace
// TODO : This should be moved closer to Deployment domain/package
func (k *Kube) GrantAppAdminAccessToRuntimeSecrets(namespace string, registration *radixv1.RadixRegistration, component *radixv1.RadixDeployComponent) error {
	if component.Secrets == nil || len(component.Secrets) <= 0 {
		return nil
	}

	role := roleAppAdminSecrets(registration, component)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	rolebinding := rolebindingAppAdminSecrets(registration, role)

	return k.ApplyRoleBinding(namespace, rolebinding)
}

// GrantAccessToCICDLogs Grants access to pipeline logs
// TODO : This should be moved closer to Application domain/package
func (k *Kube) GrantAccessToCICDLogs(registration *radixv1.RadixRegistration) error {
	namespace := utils.GetAppNamespace(registration.Name)
	subjects := getRoleBindingGroups(registration.Spec.AdGroups)
	clusterRoleName := "radix-app-admin"

	roleBinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: registration.Name,
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
// TODO : This should be moved closer to Application domain/package
func (k *Kube) ApplyRbacRadixRegistration(registration *radixv1.RadixRegistration) error {
	namespace := "default"
	role := RrUserRole(registration)
	rolebinding := rrRoleBinding(registration, role)

	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	return k.ApplyRoleBinding(namespace, rolebinding)
}

// ApplyRbacOnPipelineRunner Grants access to radix pipeline
// TODO : This should be moved closer to Application domain/package
func (k *Kube) ApplyRbacOnPipelineRunner(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	err := k.givePipelineAccessToRR(registration, serviceAccount)
	if err != nil {
		return err
	}

	err = k.givePipelineAccessToAppNamespace(registration, serviceAccount)
	if err != nil {
		return err
	}

	return k.givePipelineAccessToDefaultNamespace(registration, serviceAccount)
}

func (k *Kube) ApplyRoleBinding(namespace string, rolebinding *auth.RoleBinding) error {
	logger = logger.WithFields(log.Fields{"roleBinding": rolebinding.ObjectMeta.Name})

	logger.Infof("Apply rolebinding %s", rolebinding.Name)

	_, err := k.kubeClient.RbacV1().RoleBindings(namespace).Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		_, err = k.kubeClient.RbacV1().RoleBindings(namespace).Update(rolebinding)
		logger.Infof("Rolebinding %s already exists. Updating", rolebinding.Name)
	}

	if err != nil {
		logger.Errorf("Failed to save roleBinding in [%s]: %v", namespace, err)
		return err
	}

	logger.Infof("Created roleBinding %s in %s", rolebinding.Name, namespace)
	return nil
}

func (k *Kube) ApplyClusterRoleBinding(rolebinding *auth.ClusterRoleBinding) error {
	logger = logger.WithFields(log.Fields{"clusterRoleBinding": rolebinding.ObjectMeta.Name})

	logger.Infof("Apply clusterrolebinding %s", rolebinding.Name)

	_, err := k.kubeClient.RbacV1().ClusterRoleBindings().Create(rolebinding)
	if errors.IsAlreadyExists(err) {
		logger.Infof("ClusterRolebinding %s already exists", rolebinding.Name)
		return nil
	}

	if err != nil {
		logger.Errorf("Failed to create clusterRoleBinding: %v", err)
		return err
	}

	logger.Infof("Created clusterRoleBinding %s", rolebinding.Name)
	return nil
}

// TODO : This should be moved closer to Application domain/package
func (k *Kube) ApplyClusterRoleToServiceAccount(roleName string, registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	ownerReference := GetOwnerReferenceOfRegistration(registration)

	rolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s", serviceAccount.Namespace, serviceAccount.Name),
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
	return k.ApplyClusterRoleBinding(rolebinding)
}

func (k *Kube) givePipelineAccessToRR(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	namespace := "default"
	role := RrPipelineRole(registration)
	rolebinding := rrPipelineRoleBinding(registration, serviceAccount, role)

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

func (k *Kube) givePipelineAccessToAppNamespace(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	namespace := utils.GetAppNamespace(registration.Name)
	rolebinding := pipelineRoleBinding(registration, serviceAccount)

	return k.ApplyRoleBinding(namespace, rolebinding)
}

func (k *Kube) givePipelineAccessToDefaultNamespace(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) error {
	rolebinding := pipelineClusterRolebinding(registration, serviceAccount)

	return k.ApplyClusterRoleBinding(rolebinding)
}

func rolebindingAppAdminSecrets(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	subjects := getRoleBindingGroups(registration.Spec.AdGroups)
	roleName := role.ObjectMeta.Name

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixApp":    registration.Name, // For backwards compatibility. Remove when cluster is migrated
				RadixAppLabel: registration.Name,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: subjects,
	}

	return rolebinding
}

func pipelineClusterRolebinding(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) *auth.ClusterRoleBinding {
	appName := registration.Name
	roleName := "radix-pipeline-runner"
	ownerReference := GetOwnerReferenceOfRegistration(registration)
	logger.Infof("Create cluster rolebinding config %s", roleName)

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

func pipelineRoleBinding(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	appName := registration.Name
	roleName := "radix-pipeline"
	logger.Infof("Create rolebinding config %s", roleName)

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

func rrPipelineRoleBinding(registration *radixv1.RadixRegistration, serviceAccount *corev1.ServiceAccount, role *auth.Role) *auth.RoleBinding {
	appName := registration.Name
	roleBindingName := role.Name
	ownerReference := GetOwnerReferenceOfRegistration(registration)
	logger.Infof("Create rolebinding config %s", roleBindingName)

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

func rrRoleBinding(registration *radixv1.RadixRegistration, role *auth.Role) *auth.RoleBinding {
	appName := registration.Name
	roleBindingName := role.Name
	logger.Infof("Create roleBinding config %s", roleBindingName)

	ownerReference := GetOwnerReferenceOfRegistrationWithName(roleBindingName, registration)
	subjects := getRoleBindingGroups(registration.Spec.AdGroups)

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

	logger.Infof("Done - create rolebinding config %s", roleBindingName)

	return rolebinding
}

func getRoleBindingGroups(groups []string) []auth.Subject {
	subjects := []auth.Subject{}
	for _, group := range groups {
		subjects = append(subjects, auth.Subject{
			Kind:     "Group",
			Name:     group,
			APIGroup: "rbac.authorization.k8s.io",
		})
	}
	return subjects
}
