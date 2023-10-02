package deployment

func (deploy *Deployment) garbageCollectRadixBatchesNoLongerInSpec() error {
	batches, err := deploy.kubeutil.ListRadixBatches(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, batch := range batches {
		componentName, ok := RadixComponentNameFromComponentLabel(batch)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.kubeutil.DeleteRadixBatch(batch.GetNamespace(), batch.GetName())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
