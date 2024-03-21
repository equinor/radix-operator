package kube

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// ListPodDisruptionBudgets lists PodDisruptionBudgets
func (kubeutil *Kube) ListPodDisruptionBudgets(namespace string) ([]*v1.PodDisruptionBudget, error) {
	list, err := kubeutil.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}
	pdbs := slice.PointersOf(list.Items).([]*v1.PodDisruptionBudget)
	return pdbs, nil
}

// MergePodDisruptionBudgets returns patch bytes between two PDBs
func MergePodDisruptionBudgets(existingPdb *v1.PodDisruptionBudget, generatedPdb *v1.PodDisruptionBudget) ([]byte, error) {
	newPdb := existingPdb.DeepCopy()
	newPdb.ObjectMeta.Labels = generatedPdb.ObjectMeta.Labels
	newPdb.ObjectMeta.Annotations = generatedPdb.ObjectMeta.Annotations
	newPdb.ObjectMeta.OwnerReferences = generatedPdb.ObjectMeta.OwnerReferences
	newPdb.Spec = generatedPdb.Spec

	oldPdbJSON, err := json.Marshal(existingPdb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old PDB object: %v", err)
	}

	newPdbJSON, err := json.Marshal(newPdb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new PDB object: %v", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldPdbJSON, newPdbJSON, v1.PodDisruptionBudget{})
	if err != nil {
		return nil, fmt.Errorf("failed to create two way merge PDB objects: %v", err)
	}

	return patchBytes, nil
}

// UpdatePodDisruptionBudget will update PodDisruptionBudgets in provided namespace
func (kubeutil *Kube) UpdatePodDisruptionBudget(namespace string, pdb *v1.PodDisruptionBudget) error {
	pdbName := pdb.Name
	existingPdb, getPdbErr := kubeutil.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), pdbName, metav1.GetOptions{})
	if getPdbErr != nil {
		return getPdbErr
	}

	patchBytes, err := MergePodDisruptionBudgets(existingPdb, pdb)
	if err != nil {
		return err
	}

	if !IsEmptyPatch(patchBytes) {
		// As of July 2022, this clause is only invoked after an update on radix-operator which alters the logic defining PDBs.
		// Under normal circumstances, PDBs are only created or deleted entirely, never updated.
		_, err := kubeutil.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).Patch(context.TODO(), pdbName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch PDB object: %w", err)
		}

		return nil
	}

	log.Debug().Msgf("No need to patch PDB: %s ", pdbName)
	return nil
}
