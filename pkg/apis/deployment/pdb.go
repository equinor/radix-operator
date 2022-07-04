package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-common/utils/errors"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	log "github.com/sirupsen/logrus"
	v12 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
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
			Labels:    map[string]string{kube.RadixComponentLabel: componentName},
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
		log.Infof("PodDisruptionBudget object %s already exists in namespace %s, updating the object now", componentName, namespace)

		pdbName := getPDBName(componentName)
		existingPdb, getPdbErr := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), pdbName, metav1.GetOptions{})
		if getPdbErr != nil {
			return getPdbErr
		}

		newPdb := existingPdb.DeepCopy()
		newPdb.ObjectMeta.Labels = pdb.ObjectMeta.Labels
		newPdb.ObjectMeta.Annotations = pdb.ObjectMeta.Annotations
		newPdb.ObjectMeta.OwnerReferences = pdb.ObjectMeta.OwnerReferences
		newPdb.Spec = pdb.Spec

		oldPdbJSON, err := json.Marshal(existingPdb)
		if err != nil {
			return fmt.Errorf("failed to marshal old PDB object: %v", err)
		}

		newPdbJSON, err := json.Marshal(newPdb)
		if err != nil {
			return fmt.Errorf("failed to marshal new PDB object: %v", err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldPdbJSON, newPdbJSON, v12.PodDisruptionBudget{})
		if err != nil {
			return fmt.Errorf("failed to create two way merge PDB objects: %v", err)
		}

		if !kube.IsEmptyPatch(patchBytes) {
			patchedPdb, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Patch(context.TODO(), pdbName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("failed to patch PDB object: %v", err)
			}
			log.Debugf("Patched PDB: %s in namespace %s", patchedPdb.Name, namespace)
		} else {
			log.Debugf("No need to patch PDB: %s ", pdbName)
		}

		//_, updatePdbErr := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Update(context.TODO(), existingPdb, metav1.UpdateOptions{})
		//if updatePdbErr != nil {
		//	return updatePdbErr
		//}
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
			// Add error
		}

		if !componentName.ExistInDeploymentSpecComponentList(deploy.radixDeployment) {
			err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdb.Name, metav1.DeleteOptions{})
			// Add info debug line.
			log.Debugf("PodDisruptionBudget object %s already exists in namespace %s, deleting the object now", componentName, namespace)
			errs = append(errs, err)
		}
	}
	return errors.Concat(errs)
}
