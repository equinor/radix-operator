package deployment

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"strconv"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	secretDefaultData          = "xx"
	azureSecureStorageProvider = "azure"
)

func (deploy *Deployment) createOrUpdateSecrets(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment) error {
	envName := deployment.Spec.Environment
	namespace := utils.GetEnvironmentNamespace(registration.Name, envName)

	log.Debugf("Apply empty secrets based on radix deployment obj")
	for _, comp := range deployment.Spec.Components {
		err := deploy.createOrUpdateSecretsForComponent(registration, deployment, &comp, namespace)
		if err != nil {
			return err
		}
	}
	for _, comp := range deployment.Spec.Jobs {
		err := deploy.createOrUpdateSecretsForComponent(registration, deployment, &comp, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createOrUpdateSecretsForComponent(registration *radixv1.RadixRegistration, deployment *radixv1.RadixDeployment, component radixv1.RadixCommonDeployComponent, namespace string) error {
	secretsToManage := make([]string, 0)

	if len(component.GetSecrets()) > 0 {
		secretName := utils.GetComponentSecretName(component.GetName())
		if !deploy.kubeutil.SecretExists(namespace, secretName) {
			err := deploy.createOrUpdateSecret(namespace, registration.Name, component.GetName(), secretName, false)
			if err != nil {
				return err
			}
		} else {
			err := deploy.removeOrphanedSecrets(namespace, secretName, component.GetSecrets())
			if err != nil {
				return err
			}
		}

		secretsToManage = append(secretsToManage, secretName)
	}

	dnsExternalAlias := component.GetDNSExternalAlias()
	if dnsExternalAlias != nil && len(dnsExternalAlias) > 0 {
		err := deploy.garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component)
		if err != nil {
			return err
		}

		// Create secrets to hold TLS certificates
		for _, externalAlias := range dnsExternalAlias {
			secretsToManage = append(secretsToManage, externalAlias)

			if deploy.kubeutil.SecretExists(namespace, externalAlias) {
				continue
			}

			err := deploy.createOrUpdateSecret(namespace, registration.Name, component.GetName(), externalAlias, true)
			if err != nil {
				return err
			}
		}
	} else {
		err := deploy.garbageCollectAllSecretsForComponentAndExternalAlias(component)
		if err != nil {
			return err
		}
	}

	volumeMountSecretsToManage, err := deploy.createOrUpdateVolumeMountSecrets(namespace, component.GetName(), component.GetVolumeMounts())
	if err != nil {
		return err
	}
	secretsToManage = append(secretsToManage, volumeMountSecretsToManage...)

	err = deploy.garbageCollectVolumeMountsSecretsNoLongerInSpecForComponent(component, secretsToManage)
	if err != nil {
		return err
	}

	err = deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, component, secretsToManage)
	clientCertificateSecretName := utils.GetComponentClientCertificateSecretName(component.GetName())
	if auth := component.GetAuthentication(); auth != nil && component.GetPublicPort() != "" && IsSecretRequiredForClientCertificate(auth.ClientCertificate) {
		if !deploy.kubeutil.SecretExists(namespace, clientCertificateSecretName) {
			if err := deploy.createClientCertificateSecret(namespace, registration.Name, component.GetName(), clientCertificateSecretName); err != nil {
				return err
			}
		}
		secretsToManage = append(secretsToManage, clientCertificateSecretName)
	} else if deploy.kubeutil.SecretExists(namespace, clientCertificateSecretName) {
		err := deploy.kubeutil.DeleteSecret(namespace, clientCertificateSecretName)
		if err != nil {
			return err
		}
	}

	secretRefsSecretNames, err := deploy.createSecretRefs(namespace, registration.Name, component)
	if err != nil {
		return err
	}
	secretsToManage = append(secretsToManage, secretRefsSecretNames...)

	err = deploy.grantAppAdminAccessToRuntimeSecrets(deployment.Namespace, registration, component, secretsToManage)
	if err != nil {
		return fmt.Errorf("failed to grant app admin access to own secrets. %v", err)
	}

	if len(secretsToManage) == 0 {
		err := deploy.garbageCollectSecretsNoLongerInSpecForComponent(component)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) createOrUpdateVolumeMountSecrets(namespace, componentName string, volumeMounts []radixv1.RadixVolumeMount) ([]string, error) {
	var volumeMountSecretsToManage []string
	for _, volumeMount := range volumeMounts {
		switch volumeMount.Type {
		case radixv1.MountTypeBlob:
			{
				secretName, accountKey, accountName := deploy.getBlobFuseCredsSecrets(namespace, componentName, volumeMount.Name)
				volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
				err := deploy.createOrUpdateVolumeMountsSecrets(namespace, componentName, secretName, accountName, accountKey)
				if err != nil {
					return nil, err
				}
			}
		case radixv1.MountTypeBlobCsiAzure, radixv1.MountTypeFileCsiAzure:
			{
				secretName, accountKey, accountName := deploy.getCsiAzureCredsSecrets(namespace, componentName, volumeMount.Name)
				volumeMountSecretsToManage = append(volumeMountSecretsToManage, secretName)
				err := deploy.createOrUpdateCsiAzureVolumeMountsSecrets(namespace, componentName, volumeMount.Name, volumeMount.Type, secretName, accountName, accountKey)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return volumeMountSecretsToManage, nil
}

func (deploy *Deployment) createSecretRefs(namespace, appName string, component radixv1.RadixCommonDeployComponent) ([]string, error) {
	var secretsToManage []string
	for _, secretRef := range component.GetSecretRefs() {
		azureKeyVaultSecretNames, err := deploy.createAzureKeyVaultSecretRefs(namespace, appName, component.GetName(), secretRef)
		if err != nil {
			return secretsToManage, err
		}
		secretsToManage = append(secretsToManage, azureKeyVaultSecretNames...)
	}
	return secretsToManage, nil
}

func (deploy *Deployment) createAzureKeyVaultSecretRefs(namespace, appName, componentName string, radixSecretRef radixv1.RadixSecretRefs) ([]string, error) {
	var secretNames []string
	for _, radixAzureKeyVault := range radixSecretRef.AzureKeyVaults {
		className := kube.GetComponentSecretProviderClassName(componentName, deploy.radixDeployment.GetName(), radixv1.RadixSecretRefTypeAzureKeyVault, radixAzureKeyVault.Name)
		secretProviderClass, err := deploy.kubeutil.GetSecretProviderClass(namespace, className)
		if err != nil {
			return secretNames, err
		}
		credsSecret, err := deploy.getOrCreateAzureKeyVaultCredsSecret(namespace, appName, componentName, radixAzureKeyVault.Name)
		if err != nil {
			return secretNames, err
		}
		secretNames = append(secretNames, credsSecret.Name)
		if secretProviderClass != nil {
			return secretNames, nil
		}

		parameters, err := getSecretProviderClassParameters(radixAzureKeyVault)
		if err != nil {
			return nil, err
		}
		secretProviderClass = buildSecretProviderClass(appName, componentName, className, deploy.radixDeployment, radixv1.RadixSecretRefTypeAzureKeyVault, radixAzureKeyVault.Name)
		secretProviderClass.Spec.Parameters = parameters
		secretProviderClass.Spec.SecretObjects = getSecretProviderClassSecretObjects(componentName, deploy.radixDeployment.GetName(), radixAzureKeyVault)

		secretProviderClass, err = deploy.kubeutil.CreateSecretProviderClass(namespace, secretProviderClass)
		if err != nil {
			return secretNames, err
		}
		if !isOwnerReference(credsSecret.ObjectMeta, secretProviderClass.ObjectMeta) {
			credsSecret.ObjectMeta.OwnerReferences = append(credsSecret.ObjectMeta.OwnerReferences, getOwnerReferenceOfSecretProviderClass(secretProviderClass))
			_, err := deploy.kubeutil.ApplySecret(namespace, credsSecret)
			if err != nil {
				return secretNames, err
			}
		}
	}
	return secretNames, nil
}

func getSecretProviderClassParameters(radixAzureKeyVault radixv1.RadixAzureKeyVault) (map[string]string, error) {
	parameterMap := make(map[string]string)
	parameterMap[secretProviderClassParameterUsePodIdentity] = "false"
	parameterMap[secretProviderClassParameterKeyVaultName] = radixAzureKeyVault.Name
	parameterMap[secretProviderClassParameterTenantId] = "3aa4a235-b6e2-48d5-9195-7fcf05b459b0"
	parameterMap[secretProviderClassParameterCloudName] = ""
	if len(radixAzureKeyVault.Items) == 0 {
		parameterMap[secretProviderClassParameterObjects] = ""
		return parameterMap, nil
	}
	var parameterObject []kube.SecretProviderClassParameterObject
	for _, item := range radixAzureKeyVault.Items {
		obj := kube.SecretProviderClassParameterObject{
			Name:     item.Name,
			Alias:    commonUtils.StringUnPtr(item.Alias),
			Version:  commonUtils.StringUnPtr(item.Version),
			Format:   commonUtils.StringUnPtr(item.Format),
			Encoding: commonUtils.StringUnPtr(item.Encoding),
		}
		if item.Type != nil {
			obj.Type = string(*item.Type)
		} else {
			obj.Type = string(radixv1.RadixAzureKeyVaultObjectTypeSecret)
		}
		parameterObject = append(parameterObject, obj)
	}
	parameterObjectArray := kube.StringArray{Array: []string{}}
	for _, keyVaultObject := range parameterObject {
		obj, err := yaml.Marshal(keyVaultObject)
		if err != nil {
			return nil, err
		}
		parameterObjectArray.Array = append(parameterObjectArray.Array, string(obj))
	}
	parameterObjectArrayString, err := yaml.Marshal(parameterObjectArray)
	if err != nil {
		return nil, err
	}
	parameterMap[secretProviderClassParameterObjects] = string(parameterObjectArrayString)
	return parameterMap, nil
}

func getSecretProviderClassSecretObjects(componentName, radixDeploymentName string, radixAzureKeyVault radixv1.RadixAzureKeyVault) []*secretsstorev1.SecretObject {
	var secretObjects []*secretsstorev1.SecretObject
	secretObjectMap := make(map[kube.SecretType]*secretsstorev1.SecretObject)
	for _, keyVaultItem := range radixAzureKeyVault.Items {
		kubeSecretType := kube.GetSecretTypeForRadixAzureKeyVault(keyVaultItem.K8sSecretType)
		secretObject, ok := secretObjectMap[kubeSecretType]
		if !ok {
			secretObject = &secretsstorev1.SecretObject{
				SecretName: kube.GetAzureKeyVaultSecretRefSecretName(componentName, radixDeploymentName, radixAzureKeyVault.Name, kubeSecretType),
				Type:       string(kubeSecretType),
			}
			secretObjectMap[kubeSecretType] = secretObject
			secretObjects = append(secretObjects, secretObject)
		}
		secretObject.Data = append(secretObject.Data, &secretsstorev1.SecretObjectData{
			ObjectName: keyVaultItem.Name,
			Key:        keyVaultItem.EnvVar,
		})
	}
	return secretObjects
}

func (deploy *Deployment) getOrCreateAzureKeyVaultCredsSecret(namespace, appName, componentName, azKeyVaultName string) (*v1.Secret, error) {
	secretName := defaults.GetCsiAzureKeyVaultCredsSecretName(componentName, azKeyVaultName)
	secret, err := deploy.kubeutil.GetSecret(namespace, secretName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if secret != nil && !errors.IsNotFound(err) {
		return secret, nil
	}
	secret = buildAzureKeyVaultCredentialsSecret(appName, componentName, secretName, azKeyVaultName)
	return deploy.kubeutil.ApplySecret(namespace, secret)
}

func buildSecretProviderClass(appName, componentName, name string, radixDeployment *radixv1.RadixDeployment, radixSecretRefType radixv1.RadixSecretRefType, secretRefName string) *secretsstorev1.SecretProviderClass {
	className := kube.GetComponentSecretProviderClassName(componentName, radixDeployment.GetName(), radixSecretRefType, secretRefName)
	return &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: className,
			Labels: map[string]string{
				kube.RadixAppLabel:           appName,
				kube.RadixComponentLabel:     componentName,
				kube.RadixDeploymentLabel:    radixDeployment.Name,
				kube.RadixSecretRefTypeLabel: string(radixSecretRefType),
				kube.RadixSecretRefNameLabel: secretRefName,
			},
			OwnerReferences: []metav1.OwnerReference{
				getOwnerReferenceOfDeployment(radixDeployment),
			},
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			Provider: azureSecureStorageProvider,
		},
	}
}

func (deploy *Deployment) getBlobFuseCredsSecrets(ns, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetBlobFuseCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(secretDefaultData)
	accountName := []byte(secretDefaultData)
	if deploy.kubeutil.SecretExists(ns, secretName) {
		oldSecret, _ := deploy.kubeutil.GetSecret(ns, secretName)
		accountKey = oldSecret.Data[defaults.BlobFuseCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.BlobFuseCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func (deploy *Deployment) getCsiAzureCredsSecrets(namespace, componentName, volumeMountName string) (string, []byte, []byte) {
	secretName := defaults.GetCsiAzureCredsSecretName(componentName, volumeMountName)
	accountKey := []byte(secretDefaultData)
	accountName := []byte(secretDefaultData)
	if deploy.kubeutil.SecretExists(namespace, secretName) {
		oldSecret, _ := deploy.kubeutil.GetSecret(namespace, secretName)
		accountKey = oldSecret.Data[defaults.CsiAzureCredsAccountKeyPart]
		accountName = oldSecret.Data[defaults.CsiAzureCredsAccountNamePart]
	}
	return secretName, accountKey, accountName
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpec() error {
	secrets, err := deploy.kubeutil.ListSecrets(deploy.radixDeployment.GetNamespace())
	if err != nil {
		return err
	}

	for _, existingSecret := range secrets {
		if existingSecret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] != "" {
			// Not handled here
			continue
		}

		componentName, ok := NewRadixComponentNameFromLabels(existingSecret)
		if !ok {
			continue
		}

		// Garbage collect if secret is labelled radix-job-type=job-scheduler and not defined in RD jobs
		garbageCollect := false
		if jobType, ok := NewRadixJobTypeFromObjectLabels(existingSecret); ok && jobType.IsJobScheduler() {
			garbageCollect = !componentName.ExistInDeploymentSpecJobList(deploy.radixDeployment)
		} else {
			// Garbage collect secret if not defined in RD components or jobs
			garbageCollect = !componentName.ExistInDeploymentSpec(deploy.radixDeployment)
		}

		if garbageCollect {
			err := deploy.deleteSecret(existingSecret)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponent(component radixv1.RadixCommonDeployComponent) error {
	secrets, err := deploy.listSecretsForComponent(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		// External alias not handled here
		if secret.ObjectMeta.Labels[kube.RadixExternalAliasLabel] != "" {
			continue
		}

		// Secrets for jobs not handled here
		if _, ok := NewRadixJobTypeFromObjectLabels(secret); ok {
			continue
		}

		log.Debugf("Delete secret %s no longer in spec for component %s", secret.Name, component.GetName())
		err = deploy.deleteSecret(secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func (deploy *Deployment) garbageCollectAllSecretsForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, true)
}

func (deploy *Deployment) garbageCollectSecretsNoLongerInSpecForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent) error {
	return deploy.garbageCollectSecretsForComponentAndExternalAlias(component, false)
}

func (deploy *Deployment) garbageCollectSecretsForComponentAndExternalAlias(component radixv1.RadixCommonDeployComponent, all bool) error {
	secrets, err := deploy.listSecretsForComponentExternalAlias(component)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		garbageCollectSecret := true

		dnsExternalAlias := component.GetDNSExternalAlias()
		if !all && dnsExternalAlias != nil {
			externalAliasForSecret := secret.Name
			for _, externalAlias := range dnsExternalAlias {
				if externalAlias == externalAliasForSecret {
					garbageCollectSecret = false
				}
			}
		}

		if garbageCollectSecret {
			log.Debugf("Delete secret %s for component %s and external alias %s", secret.Name, component.GetName(), dnsExternalAlias)
			err = deploy.deleteSecret(secret)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (deploy *Deployment) listSecretsForComponent(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(getLabelSelectorForComponent(component))
}

func (deploy *Deployment) listSecretsForComponentExternalAlias(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	return deploy.listSecrets(getLabelSelectorForExternalAlias(component))
}

func (deploy *Deployment) listSecretsForVolumeMounts(component radixv1.RadixCommonDeployComponent) ([]*v1.Secret, error) {
	blobVolumeMountSecret := getLabelSelectorForBlobVolumeMountSecret(component)
	secrets, err := deploy.listSecrets(blobVolumeMountSecret)
	if err != nil {
		return nil, err
	}
	csiAzureVolumeMountSecret := getLabelSelectorForCsiAzureVolumeMountSecret(component)
	csiSecrets, err := deploy.listSecrets(csiAzureVolumeMountSecret)
	if err != nil {
		return nil, err
	}
	secrets = append(secrets, csiSecrets...)
	return secrets, err
}

func (deploy *Deployment) listSecrets(labelSelector string) ([]*v1.Secret, error) {
	secrets, err := deploy.kubeutil.ListSecretsWithSelector(deploy.radixDeployment.GetNamespace(), &labelSelector)

	if err != nil {
		return nil, err
	}

	return secrets, err
}

func (deploy *Deployment) createOrUpdateSecret(ns, app, component, secretName string, isExternalAlias bool) error {
	secretType := v1.SecretType(kube.SecretTypeOpaque)
	if isExternalAlias {
		secretType = v1.SecretType("kubernetes.io/tls")
	}

	secret := v1.Secret{
		Type: secretType,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:           app,
				kube.RadixComponentLabel:     component,
				kube.RadixExternalAliasLabel: strconv.FormatBool(isExternalAlias),
			},
		},
	}

	if isExternalAlias {
		defaultValue := []byte(secretDefaultData)

		// Will need to set fake data in order to apply the secret. The user then need to set data to real values
		data := make(map[string][]byte)
		data["tls.crt"] = defaultValue
		data["tls.key"] = defaultValue

		secret.Data = data
	}

	_, err := deploy.kubeutil.ApplySecret(ns, &secret)
	if err != nil {
		return err
	}

	return nil
}

func buildAzureKeyVaultCredentialsSecret(app, componentName, secretName, azKeyVaultName string) *v1.Secret {
	secretType := v1.SecretType(kube.SecretTypeOpaque)
	secret := v1.Secret{
		Type: secretType,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:           app,
				kube.RadixComponentLabel:     componentName,
				kube.RadixSecretRefTypeLabel: string(radixv1.RadixSecretRefTypeAzureKeyVault),
				kube.RadixSecretRefNameLabel: azKeyVaultName,
			},
		},
	}

	data := make(map[string][]byte)
	defaultValue := []byte(secretDefaultData)
	data["clientid"] = defaultValue
	data["clientsecret"] = defaultValue

	secret.Data = data
	return &secret
}

func (deploy *Deployment) createClientCertificateSecret(ns, app, component, secretName string) error {
	secret := v1.Secret{
		Type: v1.SecretType(kube.SecretTypeOpaque),
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       app,
				kube.RadixComponentLabel: component,
			},
		},
	}

	defaultValue := []byte(secretDefaultData)

	// Will need to set fake data in order to apply the secret. The user then need to set data to real values
	data := make(map[string][]byte)
	data["ca.crt"] = defaultValue
	secret.Data = data

	_, err := deploy.kubeutil.ApplySecret(ns, &secret)
	return err
}

func (deploy *Deployment) removeOrphanedSecrets(ns, secretName string, secrets []string) error {
	secret, err := deploy.kubeutil.GetSecret(ns, secretName)
	if err != nil {
		return err
	}

	orphanRemoved := false
	for secretName := range secret.Data {
		if !slice.ContainsString(secrets, secretName) {
			delete(secret.Data, secretName)
			orphanRemoved = true
		}
	}

	if orphanRemoved {
		_, err = deploy.kubeutil.ApplySecret(ns, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func IsSecretRequiredForClientCertificate(clientCertificate *radixv1.ClientCertificate) bool {
	if clientCertificate != nil {
		certificateConfig := parseClientCertificateConfiguration(*clientCertificate)
		if *certificateConfig.PassCertificateToUpstream || *certificateConfig.Verification != radixv1.VerificationTypeOff {
			return true
		}
	}

	return false
}

//GarbageCollectSecrets delete secrets, excluding with names in the excludeSecretNames
func (deploy *Deployment) GarbageCollectSecrets(secrets []*v1.Secret, excludeSecretNames []string) error {
	for _, secret := range secrets {
		if slice.ContainsString(excludeSecretNames, secret.Name) {
			continue
		}
		err := deploy.deleteSecret(secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func (deploy *Deployment) deleteSecret(secret *v1.Secret) error {
	log.Debugf("Delete secret %s", secret.Name)
	return deploy.kubeclient.CoreV1().Secrets(deploy.radixDeployment.GetNamespace()).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
}
