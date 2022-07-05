package deployment

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils/errors"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getNumberOfReplicas(component v1.RadixCommonDeployComponent) int {
	if component.GetReplicas() != nil {
		return *component.GetReplicas()
	} else {
		return DefaultReplicas // 1
	}

}

func (deploy *Deployment) createOrUpdatePodDisruptionBudget(component v1.RadixCommonDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	replicas := getNumberOfReplicas(component)
	if replicas < 2 {
		return nil
	}

	pdbName := utils.GetPDBName(componentName)
	pdb := utils.GetPDBConfig(componentName, namespace)

	log.Debugf("creating PodDisruptionBudget object %s in namespace %s", componentName, namespace)
	_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})

	if k8serrors.IsAlreadyExists(err) {
		log.Infof("PodDisruptionBudget object %s already exists in namespace %s, updating the object now", componentName, namespace)
		err := deploy.kubeutil.UpdatePodDisruptionBudget(namespace, pdb, pdbName)
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
	// Check if replicas for component is < 2. If yes, delete PDB
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	replicas := getNumberOfReplicas(component)
	if replicas < 2 {
		_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), utils.GetPDBName(componentName), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), utils.GetPDBName(componentName), metav1.DeleteOptions{})

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
			return fmt.Errorf("could not determine component name from labels in pdb %s", utils.GetPDBName(string(componentName)))
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdb.Name, metav1.DeleteOptions{})
			log.Debugf("PodDisruptionBudget object %s already exists in namespace %s, deleting the object now", componentName, namespace)
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}
