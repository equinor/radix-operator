package commands

import (
	"fmt"

	"k8s.io/client-go/kubernetes"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/statoil/radix-operator/pkg/apis/kube"
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
	role := kube.BrigadeRole(brigadeBuildId, radixApplication, ownerReference)
	rolebinding := kube.BrigadeRoleBinding(appName, role.Name, radixRegistration.Spec.AdGroups, ownerReference)

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
