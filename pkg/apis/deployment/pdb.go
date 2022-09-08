package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getNumberOfReplicas(component v1.RadixCommonDeployComponent) int {
	if component.GetReplicas() != nil {
		return *component.GetReplicas()
	}
	return DefaultReplicas // 1

}

func componentShallHavePdb(component v1.RadixCommonDeployComponent) bool {
	horizontalScaling := component.GetHorizontalScaling()
	if horizontalScaling != nil {
		if horizontalScaling.MinReplicas == nil {
			return false
		}
		if *horizontalScaling.MinReplicas < 2 {
			return false
		}
		return true
	}
	replicas := getNumberOfReplicas(component)
	return replicas >= 2
}

func (deploy *Deployment) createOrUpdatePodDisruptionBudget(component v1.RadixCommonDeployComponent) error {
	if !componentShallHavePdb(component) {
		return nil
	}
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()
	pdb := utils.GetPDBConfig(componentName, namespace)
	pdbName := pdb.Name

	log.Debugf("creating PodDisruptionBudget object %s in namespace %s", componentName, namespace)
	_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})

	if k8serrors.IsAlreadyExists(err) {
		log.Infof("PodDisruptionBudget object %s already exists in namespace %s, updating the object now", componentName, namespace)
		err := deploy.kubeutil.UpdatePodDisruptionBudget(namespace, pdb)
		if err != nil {
			return err
		}
		log.Debugf("patched PDB: %s in namespace %s", pdbName, namespace)
	} else {
		return err
	}
	return nil
}

func (deploy *Deployment) garbageCollectPodDisruptionBudgetNoLongerInSpecForComponent(component v1.RadixCommonDeployComponent) error {
	if componentShallHavePdb(component) {
		return nil
	}
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	pdbs, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		return err
	}

	var errs []error
	for _, pdb := range pdbs.Items {
		err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdb.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Concat(errs)
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
			return fmt.Errorf("could not determine component name from labels in pdb %s", utils.GetPDBName(string(componentName)))
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdb.Name, metav1.DeleteOptions{})
			log.Debugf("PodDisruptionBudget object %s already exists in namespace %s, deleting the object now", componentName, namespace)

		}
		if err != nil {
			errs = append(errs, err)
		}

	}
	return errors.Concat(errs)
}
