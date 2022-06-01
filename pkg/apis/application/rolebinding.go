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

	appNamespace := utils.GetAppNamespace(registration.Name)

	adGroups, err := GetAdGroups(registration)
	if err != nil {
		return err
	}

	subjects := kube.GetRoleBindingGroups(adGroups)

	if app.registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(registration.Name),
			Namespace: appNamespace,
		})
	}

	roleBinding := kube.GetRolebindingToClusterRoleForSubjects(registration.Name, defaults.AppAdminRoleName, subjects)
	return k.ApplyRoleBinding(appNamespace, roleBinding)
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
func (app Application) applyPlatformUserRoleToMachineUser(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil
	clusterrolebinding := app.machineUserBinding(serviceAccount)
	return k.ApplyClusterRoleBinding(clusterrolebinding)
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

	err = app.givePipelineAccessToAppNamespace(serviceAccount)
	if err != nil {
		return err
	}

	return app.givePipelineAccessToDefaultNamespace(serviceAccount)
}

func (app Application) applyRbacOnRadixTekton() error {
	serviceAccount, err := app.applyRadixTektonServiceAccount()
	if err != nil {
		return err
	}

	err = app.givePipelineAccessToRR(serviceAccount, "radix-tekton-rr")
	if err != nil {
		return err
	}

	return app.giveRadixTektonAccessToAppNamespace(serviceAccount)
}

func (app Application) applyRbacOnScanImageRunner() error {
	serviceAccount, err := app.applyScanImageServiceAccount()
	if err != nil {
		return err
	}

	return app.giveScanImageRunnerAccessToAppNamespace(serviceAccount)
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

func (app Application) giveScanImageRunnerAccessToAppNamespace(serviceAccount *corev1.ServiceAccount) error {
	k := app.kubeutil
	registration := app.registration

	namespace := utils.GetAppNamespace(registration.Name)

	// create role
	role := app.scanImageRunnerRole()
	err := k.ApplyRole(namespace, role)
	if err != nil {
		return err
	}

	// Create role binding
	rolebinding := app.scanImageRunnerRoleBinding(serviceAccount)
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
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.PipelineRunnerRoleName,
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
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.PipelineRoleName,
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
	logger.Debugf("Create rolebinding config %s", defaults.RadixTektonRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.RadixTektonRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.RadixTektonRoleName,
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

func (app Application) scanImageRunnerRoleBinding(serviceAccount *corev1.ServiceAccount) *auth.RoleBinding {
	registration := app.registration
	appName := registration.Name
	logger.Debugf("Create rolebinding config %s", defaults.ScanImageRunnerRoleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.ScanImageRunnerRoleName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     defaults.ScanImageRunnerRoleName,
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
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
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

func (app Application) rrClusterroleBinding(clusterrole *auth.ClusterRole) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := clusterrole.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)

	ownerReference := app.getOwnerReference()

	adGroups, _ := GetAdGroups(registration)
	subjects := kube.GetRoleBindingGroups(adGroups)

	if app.registration.Spec.MachineUser {
		subjects = append(subjects, auth.Subject{
			Kind:      "ServiceAccount",
			Name:      defaults.GetMachineUserRoleName(registration.Name),
			Namespace: utils.GetAppNamespace(registration.Name),
		})
	}

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
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

func (app Application) machineUserBinding(serviceAccount *corev1.ServiceAccount) *auth.ClusterRoleBinding {
	registration := app.registration
	appName := registration.Name
	clusterroleBindingName := serviceAccount.Name
	logger.Debugf("Create clusterrolebinding config %s", clusterroleBindingName)

	ownerReference := app.getOwnerReference()

	subjects := []auth.Subject{{
		Kind:      "ServiceAccount",
		Name:      defaults.GetMachineUserRoleName(registration.Name),
		Namespace: utils.GetAppNamespace(registration.Name),
	}}

	kube.GetRolebindingToClusterRoleForSubjects(appName, defaults.AppAdminEnvironmentRoleName, subjects)

	clusterrolebinding := &auth.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterroleBindingName,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
			},
			OwnerReferences: ownerReference,
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     defaults.PlatformUserRoleName,
		},
		Subjects: subjects,
	}

	logger.Debugf("Done - create clusterrolebinding config %s", clusterroleBindingName)
	return clusterrolebinding
}

func rolebindingAppAdminToMachineUserToken(appName string, adGroups []string, role *auth.Role) *auth.RoleBinding {
	roleName := role.ObjectMeta.Name
	subjects := kube.GetRoleBindingGroups(adGroups)

	// Add machine user to subjects
	subjects = append(subjects, auth.Subject{
		Kind:      "ServiceAccount",
		Name:      defaults.GetMachineUserRoleName(appName),
		Namespace: utils.GetAppNamespace(appName),
	})

	return kube.GetRolebindingToRoleForSubjectsWithLabels(appName, roleName, subjects, role.Labels)
}
