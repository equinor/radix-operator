package commands

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/statoil/radix-operator/pkg/apis/kube"
	"github.com/statoil/radix-operator/pkg/apis/radix/v1"
	auth "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const rbacBrigadeObjUsage = `Creates role and rolebinding so users in RadixRegistartion.AdGroups can get brigade objects used during deployment of their applications`

var (
	brigadeBuildId string
	fileName       string
	namespace      string
)

func init() {
	rbacCreate.Flags().StringVarP(&brigadeBuildId, "buildId", "b", "", "Brigade project build id")
	rbacCreate.Flags().StringVarP(&fileName, "filename", "f", "", "Radix application yaml file")
	rbacCreate.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace the brigade project is run. Uses 'default' if empty")
	rbac.AddCommand(rbacCreate)
}

var rbacCreate = &cobra.Command{
	Use:   "apply",
	Short: "apply RBAC for brigade deployment objects",
	Long:  rbacBrigadeObjUsage,
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyRbacForBrigadeDeployment()
	},
}

func applyRbacForBrigadeDeployment() error {
	log.Infof("Start creating rbac for brigade objects - ns: %s, buildId: %s, filename: %s", namespace, brigadeBuildId, fileName)
	kubeClient, _ := kubeClient()
	kubeutil, _ := kube.New(kubeClient)

	radixApplication := getRadixApplication(fileName)
	appName := radixApplication.Name
	radixRegistration, _ := getRadixRegistration(namespace, appName)

	ownerReference := getBrigadeWorkerAsOwnerReference(kubeClient, brigadeBuildId, namespace)
	role := createRole(brigadeBuildId, radixApplication, ownerReference)
	rolebinding := createRolebinding(appName, role.Name, radixRegistration.Spec.AdGroups, ownerReference)

	kubeutil.ApplyRole(namespace, role)
	kubeutil.ApplyRoleBinding(namespace, rolebinding)

	log.Infof("Done creating rbac for brigade objects - ns: %s, buildId: %s, filename: %s", namespace, brigadeBuildId, fileName)
	return nil
}

func getBrigadeWorkerAsOwnerReference(kubeClient *kubernetes.Clientset, brigadeBuildId, namespace string) metav1.OwnerReference {
	brigadeWorkerId := fmt.Sprintf("brigade-worker-%s", brigadeBuildId)
	brigadeWorkerPod, _ := kubeClient.CoreV1().Pods(namespace).Get(brigadeWorkerId, metav1.GetOptions{})
	ownerReferene := kube.GetOwnerReference(brigadeWorkerId, "pod", brigadeWorkerPod.UID)

	return ownerReferene
}

func createRole(brigadeBuildId string, radixAppliation *v1.RadixApplication, owner metav1.OwnerReference) *auth.Role {
	appName := radixAppliation.Name
	roleName := fmt.Sprintf("radix-brigade-%s-%s", appName, brigadeBuildId)
	resourceNames := getResourceNames(brigadeBuildId, radixAppliation.Spec.Components, radixAppliation.Spec.Environments)

	log.Infof("Creating role config %s", roleName)

	role := &auth.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			Labels: map[string]string{
				"radixBrigade": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		Rules: []auth.PolicyRule{
			{
				APIGroups:     []string{"*"},
				Resources:     []string{"secrets", "pods", "pods/log"},
				ResourceNames: resourceNames,
				Verbs:         []string{"get"},
			},
		},
	}
	log.Infof("Done - creating role config %s", roleName)

	return role
}

func createRolebinding(appName, roleName string, adGroups []string, owner metav1.OwnerReference) *auth.RoleBinding {
	subjects := kube.GetRoleBindingGroups(adGroups)
	roleBindingName := fmt.Sprintf("%s-binding", roleName)

	rolebinding := &auth.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: roleBindingName,
			Labels: map[string]string{
				"radixBrigade": appName,
			},
			OwnerReferences: []metav1.OwnerReference{
				owner,
			},
		},
		RoleRef: auth.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: subjects,
	}

	log.Infof("Done - create rolebinding config %s", roleBindingName)

	return rolebinding
}

func getResourceNames(brigadeBuildId string, components []v1.RadixComponent, environments []v1.Environment) []string {
	resourceNames := []string{
		fmt.Sprintf("brigade-worker-%s", brigadeBuildId),
		fmt.Sprintf("config-%s", brigadeBuildId),
	}

	for _, env := range environments {
		resourceNames = append(resourceNames, fmt.Sprintf("build-%s-%s", env.Name, brigadeBuildId))
		for _, component := range components {
			resourceNames = append(resourceNames, fmt.Sprintf("deploy-%s-%s-%s", env.Name, component.Name, brigadeBuildId))
		}
	}
	return resourceNames
}
