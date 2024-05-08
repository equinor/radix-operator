package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
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
	replicas := getNumberOfReplicas(component)
	if replicas == 0 {
		return false
	}
	horizontalScaling := component.GetHorizontalScaling()
	if horizontalScaling != nil {
		return horizontalScaling.MinReplicas != nil &&
			*horizontalScaling.MinReplicas > 1
	}
	return replicas > 1
}

func (deploy *Deployment) createOrUpdatePodDisruptionBudget(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	if !componentShallHavePdb(component) {
		return nil
	}
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()
	pdb := utils.GetPDBConfig(componentName, namespace)
	pdbName := pdb.Name

	log.Ctx(ctx).Debug().Msgf("creating PodDisruptionBudget object %s in namespace %s", componentName, namespace)
	_, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{})

	if k8serrors.IsAlreadyExists(err) {
		log.Ctx(ctx).Info().Msgf("PodDisruptionBudget object %s already exists in namespace %s, updating the object now", componentName, namespace)
		err := deploy.kubeutil.UpdatePodDisruptionBudget(ctx, namespace, pdb)
		if err != nil {
			return err
		}
		log.Ctx(ctx).Debug().Msgf("patched PDB: %s in namespace %s", pdbName, namespace)
	} else {
		return err
	}
	return nil
}

func (deploy *Deployment) garbageCollectPodDisruptionBudgetNoLongerInSpecForComponent(ctx context.Context, component v1.RadixCommonDeployComponent) error {
	if componentShallHavePdb(component) {
		return nil
	}
	namespace := deploy.radixDeployment.Namespace
	componentName := component.GetName()

	pdbs, err := deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", kube.RadixComponentLabel, componentName),
	})
	if err != nil {
		return err
	}

	var errs []error
	for _, pdb := range pdbs.Items {
		err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, pdb.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (deploy *Deployment) garbageCollectPodDisruptionBudgetsNoLongerInSpec(ctx context.Context) error {
	namespace := deploy.radixDeployment.Namespace

	// List PDBs
	pdbs, err := deploy.kubeutil.ListPodDisruptionBudgets(ctx, namespace)

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
			err = deploy.kubeclient.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, pdb.Name, metav1.DeleteOptions{})
			log.Ctx(ctx).Debug().Msgf("PodDisruptionBudget object %s already exists in namespace %s, deleting the object now", componentName, namespace)

		}
		if err != nil {
			errs = append(errs, err)
		}

	}
	return errors.Join(errs...)
}
