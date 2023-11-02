package test

import (
	"context"
	"os"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

const dnsZone = "dev.radix.equinor.com"

// Utils Instance variables
type Utils struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
	kubeUtil    *kube.Kube
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface,
	secretproviderclient secretProviderClient.Interface) Utils {
	kubeUtil, _ := kube.New(client, radixclient, secretproviderclient)
	return Utils{
		client:      client,
		radixclient: radixclient,
		kubeUtil:    kubeUtil,
	}
}

func (tu *Utils) GetKubeUtil() *kube.Kube {
	return tu.kubeUtil
}

// ApplyRegistration Will help persist an application registration
func (tu *Utils) ApplyRegistration(registrationBuilder utils.RegistrationBuilder) (*radixv1.RadixRegistration, error) {
	rr := registrationBuilder.BuildRR()

	_, err := tu.radixclient.RadixV1().RadixRegistrations().Create(context.TODO(), rr, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return tu.ApplyRegistrationUpdate(registrationBuilder)
		}
		return rr, err
	}

	return rr, nil
}

// ApplyRegistrationUpdate Will help update a registration
func (tu *Utils) ApplyRegistrationUpdate(registrationBuilder utils.RegistrationBuilder) (*radixv1.RadixRegistration, error) {
	rr := registrationBuilder.BuildRR()

	rrPrev, err := tu.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), rr.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rr.Status = rrPrev.Status

	rr, err = tu.radixclient.RadixV1().RadixRegistrations().Update(context.TODO(), rr, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return rr, nil
}

// ApplyApplication Will help persist an application
func (tu *Utils) ApplyApplication(applicationBuilder utils.ApplicationBuilder) (*radixv1.RadixApplication, error) {
	regBuilder := applicationBuilder.GetRegistrationBuilder()
	var rr *radixv1.RadixRegistration

	if regBuilder != nil {
		rr, _ = tu.ApplyRegistration(regBuilder)
	}

	ra := applicationBuilder.BuildRA()
	appNamespace := CreateAppNamespace(tu.client, ra.GetName())
	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Create(context.TODO(), ra, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return tu.ApplyApplicationUpdate(applicationBuilder)
		}

		return ra, err
	}
	ra.ObjectMeta.UID = uuid.NewUUID() // imitate new UID, assigned by Kubernetes
	// Note: rr may be nil if not found but that is fine
	for _, env := range ra.Spec.Environments {
		tu.ApplyEnvironment(utils.NewEnvironmentBuilder().
			WithAppName(ra.GetName()).
			WithAppLabel().
			WithEnvironmentName(env.Name).
			WithRegistrationOwner(rr).
			WithOrphaned(false))
	}

	return ra, nil
}

// ApplyApplicationUpdate Will help update an application
func (tu *Utils) ApplyApplicationUpdate(applicationBuilder utils.ApplicationBuilder) (*radixv1.RadixApplication, error) {
	ra := applicationBuilder.BuildRA()
	appNamespace := utils.GetAppNamespace(ra.GetName())

	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Update(context.TODO(), ra, metav1.UpdateOptions{})
	if err != nil {
		return ra, err
	}

	var rr *radixv1.RadixRegistration
	regBuilder := applicationBuilder.GetRegistrationBuilder()
	if regBuilder != nil {
		rr, err = tu.ApplyRegistration(regBuilder)
	} else {
		rr, err = tu.radixclient.RadixV1().RadixRegistrations().Get(context.TODO(), ra.GetName(), metav1.GetOptions{})
	}
	if err != nil && !errors.IsNotFound(err) {
		return ra, err
	}

	// Note: rr may be nil if not found but that is fine
	for _, env := range ra.Spec.Environments {
		tu.ApplyEnvironment(utils.NewEnvironmentBuilder().
			WithAppName(ra.GetName()).
			WithAppLabel().
			WithEnvironmentName(env.Name).
			WithRegistrationOwner(rr))
	}

	return ra, nil
}

// ApplyDeployment Will help persist a deployment
func (tu *Utils) ApplyDeployment(deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	envs := make(map[string]struct{})
	if deploymentBuilder.GetApplicationBuilder() != nil {
		ra, _ := tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
		for _, env := range ra.Spec.Environments {
			envs[env.Name] = struct{}{}
		}
	}

	rd := deploymentBuilder.BuildRD()
	log.Debugf("%s", rd.GetObjectMeta().GetCreationTimestamp())
	envs[rd.Namespace] = struct{}{}
	for env := range envs {
		CreateEnvNamespace(tu.client, rd.Spec.AppName, env)
	}

	envNamespace := rd.Namespace

	tu.applyRadixDeploymentEnvVarsConfigMaps(rd)
	newRd, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return newRd, nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdate(deploymentBuilder utils.DeploymentBuilder) (*radixv1.RadixDeployment, error) {
	rd := deploymentBuilder.BuildRD()
	envNamespace := utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)

	rdPrev, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Get(context.TODO(), rd.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rd.Status = rdPrev.Status

	rd, err = tu.radixclient.RadixV1().RadixDeployments(envNamespace).Update(context.TODO(), rd, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return rd, nil
}

// ApplyJob Will help persist a radixjob
func (tu *Utils) ApplyJob(jobBuilder utils.JobBuilder) (*radixv1.RadixJob, error) {
	if jobBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(jobBuilder.GetApplicationBuilder())
	}

	rj := jobBuilder.BuildRJ()

	appNamespace := CreateAppNamespace(tu.client, rj.Spec.AppName)
	newRj, err := tu.radixclient.RadixV1().RadixJobs(appNamespace).Create(context.TODO(), rj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return newRj, nil
}

// ApplyJobUpdate Will help update a radixjob
func (tu *Utils) ApplyJobUpdate(jobBuilder utils.JobBuilder) (*radixv1.RadixJob, error) {
	rj := jobBuilder.BuildRJ()

	appNamespace := CreateAppNamespace(tu.client, rj.Spec.AppName)

	rjPrev, err := tu.radixclient.RadixV1().RadixJobs(appNamespace).Get(context.TODO(), rj.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rj.Status = rjPrev.Status

	rj, err = tu.radixclient.RadixV1().RadixJobs(appNamespace).Update(context.TODO(), rj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return rj, nil
}

// ApplyEnvironment Will help persist a RadixEnvironment
func (tu *Utils) ApplyEnvironment(environmentBuilder utils.EnvironmentBuilder) (*radixv1.RadixEnvironment, error) {
	re := environmentBuilder.BuildRE()
	log.Debugf("%s", re.GetObjectMeta().GetCreationTimestamp())

	newRe, err := tu.radixclient.RadixV1().RadixEnvironments().Create(context.TODO(), re, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return tu.ApplyEnvironmentUpdate(environmentBuilder)
		}
		return nil, err
	}

	return newRe, nil
}

// ApplyEnvironmentUpdate Will help update a RadixEnvironment
func (tu *Utils) ApplyEnvironmentUpdate(environmentBuilder utils.EnvironmentBuilder) (*radixv1.RadixEnvironment, error) {
	re := environmentBuilder.BuildRE()

	rePrev, err := tu.radixclient.RadixV1().RadixEnvironments().Get(context.TODO(), re.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	re.Status = rePrev.Status

	re, err = tu.radixclient.RadixV1().RadixEnvironments().Update(context.TODO(), re, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return re, nil
}

// SetRequiredEnvironmentVariables  Sets the required environment
// variables needed for the operator to run properly
func SetRequiredEnvironmentVariables() {
	os.Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, "1234-5678-91011")
	os.Setenv(defaults.OperatorDNSZoneEnvironmentVariable, dnsZone)
	os.Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, "app.dev.radix.equinor.com")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	os.Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	os.Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler:main-latest")
	os.Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
	os.Setenv(defaults.OperatorTenantIdEnvironmentVariable, "01234567-8901-2345-6789-012345678901")
	os.Setenv(defaults.ContainerRegistryEnvironmentVariable, "any.container.registry")
	os.Setenv(defaults.AppContainerRegistryEnvironmentVariable, "any.app.container.registry")
}

// CreateClusterPrerequisites Will do the needed setup which is part of radix boot
func (tu *Utils) CreateClusterPrerequisites(clustername, egressIps, subscriptionId string) {
	SetRequiredEnvironmentVariables()

	tu.client.CoreV1().Secrets(corev1.NamespaceDefault).Create(
		context.TODO(),
		&corev1.Secret{
			Type: "Opaque",
			ObjectMeta: metav1.ObjectMeta{
				Name:      "radix-known-hosts-git",
				Namespace: corev1.NamespaceDefault,
			},
			Data: map[string][]byte{
				"known_hosts": []byte("abcd"),
			},
		},
		metav1.CreateOptions{})

	tu.client.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(
		context.TODO(),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "radix-config",
				Namespace: corev1.NamespaceDefault,
			},
			Data: map[string]string{
				"clustername":            clustername,
				"clusterActiveEgressIps": egressIps,
				"subscriptionId":         subscriptionId,
			},
		},
		metav1.CreateOptions{})
}

// CreateAppNamespace Helper method to creat app namespace
func CreateAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := utils.GetAppNamespace(appName)
	createNamespace(kubeclient, appName, "app", ns)
	return ns
}

// CreateEnvNamespace Helper method to creat env namespace
func CreateEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := utils.GetEnvironmentNamespace(appName, environment)
	createNamespace(kubeclient, appName, environment, ns)
	return ns
}

func createNamespace(kubeclient kubernetes.Interface, appName, envName, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
			Labels: map[string]string{
				kube.RadixAppLabel: appName,
				kube.RadixEnvLabel: envName,
			},
		},
	}

	kubeclient.CoreV1().Namespaces().Create(context.TODO(), &namespace, metav1.CreateOptions{})
}

// IntPtr Helper function to get the pointer of an int
func IntPtr(i int) *int {
	return &i
}

func (tu *Utils) applyRadixDeploymentEnvVarsConfigMaps(rd *radixv1.RadixDeployment) map[string]*corev1.ConfigMap {
	envVarConfigMapsMap := map[string]*corev1.ConfigMap{}
	for _, deployComponent := range rd.Spec.Components {
		envVarConfigMapsMap[deployComponent.GetName()] = tu.ensurePopulatedEnvVarsConfigMaps(rd, &deployComponent)
	}
	for _, deployJoyComponent := range rd.Spec.Jobs {
		envVarConfigMapsMap[deployJoyComponent.GetName()] = tu.ensurePopulatedEnvVarsConfigMaps(rd, &deployJoyComponent)
	}
	return envVarConfigMapsMap
}

func (tu *Utils) ensurePopulatedEnvVarsConfigMaps(rd *radixv1.RadixDeployment, deployComponent radixv1.RadixCommonDeployComponent) *corev1.ConfigMap {
	initialEnvVarsConfigMap, _, _ := tu.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(rd.GetNamespace(), rd.Spec.AppName,
		deployComponent.GetName())
	desiredConfigMap := initialEnvVarsConfigMap.DeepCopy()
	for envVarName, envVarValue := range deployComponent.GetEnvironmentVariables() {
		if utils.IsRadixEnvVar(envVarName) {
			continue
		}
		desiredConfigMap.Data[envVarName] = envVarValue
	}
	_ = tu.kubeUtil.ApplyConfigMap(rd.GetNamespace(), initialEnvVarsConfigMap, desiredConfigMap)
	return desiredConfigMap
}

// GetRadixAzureKeyVaultObjectTypePtr Gets pointer to RadixAzureKeyVaultObjectType
func GetRadixAzureKeyVaultObjectTypePtr(objectType radixv1.RadixAzureKeyVaultObjectType) *radixv1.RadixAzureKeyVaultObjectType {
	return &objectType
}

// GetAzureKeyVaultTypeSecrets Gets secrets with kube.RadixSecretRefTypeLabel and value v1.RadixSecretRefTypeAzureKeyVault
func GetAzureKeyVaultTypeSecrets(secrets *corev1.SecretList) *corev1.SecretList {
	var azureKeyVaultSecrets []corev1.Secret
	for _, secret := range secrets.Items {
		if label, ok := secret.ObjectMeta.Labels[kube.RadixSecretRefTypeLabel]; ok && label == string(radixv1.RadixSecretRefTypeAzureKeyVault) {
			azureKeyVaultSecrets = append(azureKeyVaultSecrets, secret)
		}
	}
	return &corev1.SecretList{Items: azureKeyVaultSecrets}
}

func AssertError(t *testing.T, expectedError string, err error) {
	switch {
	case len(expectedError) > 0 && err == nil:
		t.Errorf("missing expected OnSync() error %s", expectedError)
		return
	case len(expectedError) == 0 && err != nil:
		t.Errorf("unexpected OnSync() error = %v", err)
		return
	case len(expectedError) > 0 && err != nil:
		require.Equal(t, expectedError, err.Error())
	}
}

// RegisterRadixDNSAliases Register RadixDNSAliases
func RegisterRadixDNSAliases(radixClient radixclient.Interface, radixDNSAliasesMap map[string]radixv1.RadixDNSAliasSpec) error {
	for domain, aliasesSpec := range radixDNSAliasesMap {
		err := RegisterRadixDNSAliasBySpec(radixClient, domain, aliasesSpec)
		if err != nil {
			return err
		}
	}
	return nil
}

// RegisterRadixDNSAlias Register RadixDNSAlias by properties
func RegisterRadixDNSAlias(radixClient radixclient.Interface, appName, componentName, envName, domain string, port int32) error {
	return RegisterRadixDNSAliasBySpec(radixClient, domain, radixv1.RadixDNSAliasSpec{
		AppName:     appName,
		Environment: envName,
		Component:   componentName,
		Port:        port,
	})
}

// RegisterRadixDNSAliasBySpec Register RadixDNSAlias by its spec
func RegisterRadixDNSAliasBySpec(radixClient radixclient.Interface, domain string, aliasesSpec radixv1.RadixDNSAliasSpec) error {
	_, err := radixClient.RadixV1().RadixDNSAliases().Create(context.Background(),
		&radixv1.RadixDNSAlias{
			ObjectMeta: metav1.ObjectMeta{
				Name:   domain,
				Labels: map[string]string{kube.RadixAppLabel: aliasesSpec.AppName},
			},
			Spec: aliasesSpec,
		}, metav1.CreateOptions{})
	return err
}
