package deployment

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (deploy *Deployment) garbageCollectRadixBatchesNoLongerInSpec() error {
	batches, err := deploy.radixclient.RadixV1().RadixBatches(deploy.radixDeployment.GetNamespace()).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}

	for _, batch := range batches.Items {
		componentName, ok := RadixComponentNameFromComponentLabel(&batch)
		if !ok {
			continue
		}

		if !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment) {
			err = deploy.radixclient.RadixV1().RadixBatches(batch.GetNamespace()).Delete(context.Background(), batch.GetName(), v1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
