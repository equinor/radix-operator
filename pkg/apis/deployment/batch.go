package deployment

import "context"

func (deploy *Deployment) garbageCollectRadixBatchesNoLongerInSpec(ctx context.Context) error {
	batches, err := deploy.kubeutil.ListRadixBatches(ctx, deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, batch := range batches {
		componentName, ok := RadixComponentNameFromComponentLabel(batch)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.kubeutil.DeleteRadixBatch(ctx, batch.GetNamespace(), batch.GetName())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
