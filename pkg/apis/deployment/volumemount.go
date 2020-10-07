package deployment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	blobfuseDriver      = "azure/blobfuse"
	blobfusecreds       = "%s-blobfusecreds" // <componentname>-blobfusecreds
	defaultData         = "xx"
	defaultMountOptions = "--file-cache-timeout-in-seconds=120"
	tempPath            = "/tmp/%s-%s" // /tmp/<namespace>-<componentname>
)

func (deploy *Deployment) getVolumeMounts(deployComponent *radixv1.RadixDeployComponent) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)

	if len(deployComponent.VolumeMounts) > 0 {
		for _, volumeMount := range deployComponent.VolumeMounts {
			if volumeMount.Type == radixv1.MountTypeBlob {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      volumeMount.Name,
					MountPath: volumeMount.Path,
				})
			}
		}
	}

	return volumeMounts
}

func (deploy *Deployment) getVolumes(deployComponent *radixv1.RadixDeployComponent) []corev1.Volume {
	volumes := make([]corev1.Volume, 0)

	if len(deployComponent.VolumeMounts) > 0 {
		namespace := deploy.radixDeployment.Namespace

		for _, volumeMount := range deployComponent.VolumeMounts {
			if volumeMount.Type == radixv1.MountTypeBlob {
				flexVolumeOptions := make(map[string]string)
				flexVolumeOptions["container"] = volumeMount.Name
				flexVolumeOptions["mountoptions"] = defaultMountOptions
				flexVolumeOptions["tmppath"] = fmt.Sprintf(tempPath, namespace, deployComponent.Name)

				volumes = append(volumes, corev1.Volume{
					Name: volumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						FlexVolume: &corev1.FlexVolumeSource{
							Driver:  blobfuseDriver,
							Options: flexVolumeOptions,
							SecretRef: &corev1.LocalObjectReference{
								Name: fmt.Sprintf(blobfusecreds, deployComponent.Name),
							},
						},
					},
				})
			}
		}
	}

	return volumes
}

func (deploy *Deployment) createOrUpdateVolumeMounts(deployComponent radixv1.RadixDeployComponent) error {
	namespace := deploy.radixDeployment.Namespace

	// The rest is part of the deployment spec
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
	secretName := fmt.Sprintf(blobfusecreds, component.Name)
	existingSecret, _ := deploy.kubeutil.GetSecret(namespace, secretName)
	if existingSecret != nil {
		deploy.kubeutil.DeleteSecret(namespace, secretName)
	}

	return nil
}
