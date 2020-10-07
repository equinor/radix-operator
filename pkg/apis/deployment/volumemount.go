package deployment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	blobfusecreds = "%s-blobfusecreds"
	defaultData   = "xx"
)

func (deploy *Deployment) createOrUpdateVolumeMounts(deployComponent radixv1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace

	for _, volumeMount := range deployComponent.VolumeMounts {
		if volumeMount.Type == radixv1.MountTypeBlob {
			blobfusecredsSecret := v1.Secret{
				Type: "azure/blobfuse",
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf(blobfusecreds, deployComponent.Name),
					Labels: map[string]string{
						kube.RadixAppLabel:       deploy.registration.Name,
						kube.RadixComponentLabel: deployComponent.Name,
					},
				},
			}

			defaultValue := []byte(tlsSecretDefaultData)

			// Will need to set fake data in order to apply the secret. The user then need to set data to real values
			data := make(map[string][]byte)
			data["accountkey"] = defaultValue
			data["accountname"] = defaultValue

			blobfusecredsSecret.Data = data

			_, err := deploy.kubeutil.ApplySecret(namespace, &blobfusecredsSecret)
			if err != nil {
				return err
			}

			return nil
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectVolumeMountsNoLongerInSpecForComponent(component radixv1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace
	existingSecret, _ := deploy.kubeutil.GetSecret(namespace, fmt.Sprintf(blobfusecreds, component.Name))
	if existingSecret != nil {
		deploy.kubeutil.DeleteSecret(namespace, blobfusecreds)
	}

	return nil
}
