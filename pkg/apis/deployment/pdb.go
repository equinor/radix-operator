package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	v12 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func getPDBName(componentName string) string {
	return fmt.Sprintf("%s-pdb", componentName)
}

func getPDBConfig(componentName string, namespace string) *v12.PodDisruptionBudget {
	pdb := &v12.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPDBName(componentName),
			Namespace: namespace,
		},
		Spec: v12.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				IntVal: 1,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{kube.RadixComponentLabel: componentName},
			},
		},
	}
	return pdb
}

func getNumberOfReplicas(component v1.RadixCommonDeployComponent) int {
	if component.GetReplicas() != nil {
		return *component.GetReplicas()
	} else {
		return 1
	}

}

func (deploy *Deployment) createPodDisruptionBudget(component v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	replicas := getNumberOfReplicas(component)
	if replicas < 2 {
		return nil
	}

	pdb := getPDBConfig(componentName, namespace)

	log.Debugf("Creating PodDisruptionBudget object %s in namespace %s", componentName, namespace)
	_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		log.Debugf("PodDisruptionBudget object %s already exists in namespace %s, nothing to create", componentName, namespace)
	}
	if err != nil {
		return err
	}
	return nil
}

func (deploy *Deployment) garbageCollectPodDisruptionBudgetNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	// Check if replicas for component is < 2. If yes, delete PDB
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	replicas := getNumberOfReplicas(component)
	if replicas < 2 {
		_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), getPDBName(componentName), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), getPDBName(componentName), metav1.DeleteOptions{})

		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) garbageCollectPodDisruptionBudgetsNoLongerInSpec() error {
	namespace := deploy.radixDeployment.Namespace

	// List PDBs
	pdbs, err := deploy.kubeutil.ListPodDisruptionBudgets(namespace)

	if err != nil {
		return err
	}

	var errs []error

	// Iterate existing PDBs. Check if any of them belong to components which no longer exist
	for _, pdb := range pdbs {
		componentName, ok := RadixComponentNameFromComponentLabel(pdb)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdb.Name, metav1.DeleteOptions{})
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}
