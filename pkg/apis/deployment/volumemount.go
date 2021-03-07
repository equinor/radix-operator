package deployment

import (
	"fmt"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	blobfuseDriver                      = "azure/blobfuse"
	defaultData                         = "xx"
	defaultMountOptions                 = "--file-cache-timeout-in-seconds=120"
	volumeName                          = "blobfuse-%s-%s"         // blobfuse-<componentname>-<volumename>
	blobFuseVolumeNodeMountPathTemplate = "/tmp/%s/%s/%s/%s/%s/%s" // /tmp/<namespace>/<componentname>/<environment>/<volumetype>/<volumename>/<container>
)

func GetRadixDeployComponentVolumeMounts(deployComponent radixv1.RadixCommonDeployComponent) []corev1.VolumeMount {
	return getVolumeMounts(deployComponent.GetName(), deployComponent.GetVolumeMounts())
}

func getVolumeMounts(componentName string, componentVolumeMounts *[]radixv1.RadixVolumeMount) []v1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)

	if len(*componentVolumeMounts) > 0 {
		for _, volumeMount := range *componentVolumeMounts {
			if volumeMount.Type == radixv1.MountTypeBlob {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf(volumeName, componentName, volumeMount.Name),
					MountPath: volumeMount.Path,
				})
			}
		}
	}

	return volumeMounts
}

func (deploy *Deployment) getVolumes(deployComponent radixv1.RadixCommonDeployComponent) []corev1.Volume {
	return GetVolumes(deploy.getNamespace(), deploy.radixDeployment.Spec.Environment, deployComponent.GetName(), deployComponent.GetVolumeMounts())
}

func GetVolumes(namespace string, environment string, componentName string, volumeMounts *[]radixv1.RadixVolumeMount) []v1.Volume {
	volumes := make([]corev1.Volume, 0)

	if len(*volumeMounts) > 0 {
		for _, volumeMount := range *volumeMounts {
			if volumeMount.Type == radixv1.MountTypeBlob {
				secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMount.Name)

				flexVolumeOptions := make(map[string]string)
				flexVolumeOptions["name"] = volumeMount.Name
				flexVolumeOptions["container"] = volumeMount.Container
				flexVolumeOptions["mountoptions"] = defaultMountOptions
				flexVolumeOptions["tmppath"] = fmt.Sprintf(blobFuseVolumeNodeMountPathTemplate, namespace, componentName, environment, radixv1.MountTypeBlob, volumeMount.Name, volumeMount.Container)

				volumes = append(volumes, corev1.Volume{
					Name: fmt.Sprintf(volumeName, componentName, volumeMount.Name),
					VolumeSource: corev1.VolumeSource{
						FlexVolume: &corev1.FlexVolumeSource{
							Driver:  blobfuseDriver,
							Options: flexVolumeOptions,
							SecretRef: &corev1.LocalObjectReference{
								Name: secretName,
							},
						},
					},
				})
			}
		}
	}

	return volumes
}

func (deploy *Deployment) createOrUpdateVolumeMountsSecrets(namespace, componentName, secretName string, accountName, accountKey []byte) error {
	blobfusecredsSecret := v1.Secret{
		Type: "azure/blobfuse",
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       deploy.registration.Name,
				kube.RadixComponentLabel: componentName,
				kube.RadixMountTypeLabel: string(radixv1.MountTypeBlob),
			},
		},
	}

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data[defaults.BlobFuseCredsAccountKeyPart] = accountKey
	data[defaults.BlobFuseCredsAccountNamePart] = accountName

	blobfusecredsSecret.Data = data

	_, err := deploy.kubeutil.ApplySecret(namespace, &blobfusecredsSecret)
	if err != nil {
		return err
	}

	return nil
}

func (deploy *Deployment) garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	secrets, err := deploy.listSecretsForForBlobVolumeMount(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		err = deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(secret.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
