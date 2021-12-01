package test

import (
	"context"
	"os"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const dnsZone = "dev.radix.equinor.com"

// Utils Instance variables
type Utils struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
	kubeUtil    *kube.Kube
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface) Utils {
	kubeUtil, _ := kube.New(client, radixclient)
	kubeUtil.WithSecretsProvider(secretproviderfake.NewSimpleClientset())
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
func (tu *Utils) ApplyRegistration(registrationBuilder builders.RegistrationBuilder) (*v1.RadixRegistration, error) {
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
func (tu *Utils) ApplyRegistrationUpdate(registrationBuilder builders.RegistrationBuilder) (*v1.RadixRegistration, error) {
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
func (tu *Utils) ApplyApplication(applicationBuilder builders.ApplicationBuilder) (*v1.RadixApplication, error) {

	regBuilder := applicationBuilder.GetRegistrationBuilder()
	var rr *v1.RadixRegistration

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

	// Note: rr may be nil if not found but that is fine
	for _, env := range ra.Spec.Environments {
		tu.ApplyEnvironment(builders.NewEnvironmentBuilder().
			WithAppName(ra.GetName()).
			WithAppLabel().
			WithEnvironmentName(env.Name).
			WithRegistrationOwner(rr).
			WithOrphaned(false))
	}

	return ra, nil
}

// ApplyApplicationUpdate Will help update an application
func (tu *Utils) ApplyApplicationUpdate(applicationBuilder builders.ApplicationBuilder) (*v1.RadixApplication, error) {
	ra := applicationBuilder.BuildRA()
	appNamespace := builders.GetAppNamespace(ra.GetName())

	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Update(context.TODO(), ra, metav1.UpdateOptions{})
	if err != nil {
		return ra, err
	}

	var rr *v1.RadixRegistration
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
		tu.ApplyEnvironment(builders.NewEnvironmentBuilder().
			WithAppName(ra.GetName()).
			WithAppLabel().
			WithEnvironmentName(env.Name).
			WithRegistrationOwner(rr))
	}

	return ra, nil
}

// ApplyDeployment Will help persist a deployment
func (tu *Utils) ApplyDeployment(deploymentBuilder builders.DeploymentBuilder) (*v1.RadixDeployment, error) {
	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	rd := deploymentBuilder.BuildRD()
	log.Debugf("%s", rd.GetObjectMeta().GetCreationTimestamp())

	envNamespace := CreateEnvNamespace(tu.client, rd.Spec.AppName, rd.Spec.Environment)
	tu.applyRadixDeploymentEnvVarsConfigMaps(rd)
	newRd, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return newRd, nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdate(deploymentBuilder builders.DeploymentBuilder) (*v1.RadixDeployment, error) {
	rd := deploymentBuilder.BuildRD()
	envNamespace := builders.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)

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
func (tu *Utils) ApplyJob(jobBuilder builders.JobBuilder) (*v1.RadixJob, error) {
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
func (tu *Utils) ApplyJobUpdate(jobBuilder builders.JobBuilder) (*v1.RadixJob, error) {
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
func (tu *Utils) ApplyEnvironment(environmentBuilder builders.EnvironmentBuilder) (*v1.RadixEnvironment, error) {
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
func (tu *Utils) ApplyEnvironmentUpdate(environmentBuilder builders.EnvironmentBuilder) (*v1.RadixEnvironment, error) {
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
	os.Setenv("RADIXOPERATOR_DEFAULT_USER_GROUP", "1234-5678-91011")
	os.Setenv(defaults.OperatorDNSZoneEnvironmentVariable, dnsZone)
	os.Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, "app.dev.radix.equinor.com")
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "1")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	os.Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	os.Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	os.Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler-server:main-latest")
	os.Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
}

// CreateClusterPrerequisites Will do the needed setup which is part of radix boot
func (tu *Utils) CreateClusterPrerequisites(clustername, containerRegistry string) {
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
				"clustername":       clustername,
				"containerRegistry": containerRegistry,
			},
		},
		metav1.CreateOptions{})
}

// CreateAppNamespace Helper method to creat app namespace
func CreateAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := builders.GetAppNamespace(appName)
	createNamespace(kubeclient, appName, "app", ns)
	return ns
}

// CreateEnvNamespace Helper method to creat env namespace
func CreateEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := builders.GetEnvironmentNamespace(appName, environment)
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

func (tu *Utils) applyRadixDeploymentEnvVarsConfigMaps(rd *v1.RadixDeployment) map[string]*corev1.ConfigMap {
	envVarConfigMapsMap := map[string]*corev1.ConfigMap{}
	for _, deployComponent := range rd.Spec.Components {
		envVarConfigMapsMap[deployComponent.GetName()] = tu.ensurePopulatedEnvVarsConfigMaps(rd, &deployComponent)
	}
	for _, deployJoyComponent := range rd.Spec.Jobs {
		envVarConfigMapsMap[deployJoyComponent.GetName()] = tu.ensurePopulatedEnvVarsConfigMaps(rd, &deployJoyComponent)
	}
	return envVarConfigMapsMap
}

func (tu *Utils) ensurePopulatedEnvVarsConfigMaps(rd *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) *corev1.ConfigMap {
	initialEnvVarsConfigMap, _, _ := tu.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(rd.GetNamespace(), rd.GetName(), deployComponent.GetName())
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
