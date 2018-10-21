package utils

import (
	"fmt"

	"github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/radix-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TestUtils Instance variables
type TestUtils struct {
	client              kubernetes.Interface
	radixclient         radixclient.Interface
	registrationHandler common.Handler
	deploymentHandler   common.Handler
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface, registrationHandler common.Handler, deploymentHandler common.Handler) TestUtils {
	return TestUtils{
		client:              client,
		radixclient:         radixclient,
		registrationHandler: registrationHandler,
		deploymentHandler:   deploymentHandler,
	}
}

// ApplyRegistration Will help persist an application registration
func (tu *TestUtils) ApplyRegistration(registrationBuilder utils.RegistrationBuilder) {
	rr := registrationBuilder.BuildRR()

	tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(rr)
	tu.registrationHandler.ObjectCreated(rr)
}

// ApplyApplication Will help persist an application
func (tu *TestUtils) ApplyApplication(applicationBuilder utils.ApplicationBuilder) {
	if applicationBuilder.GetRegistrationBuilder() != nil {
		tu.ApplyRegistration(applicationBuilder.GetRegistrationBuilder())
	}

	ra := applicationBuilder.BuildRA()
	appNamespace := CreateAppNamespace(tu.client, ra.GetName())
	tu.radixclient.RadixV1().RadixApplications(appNamespace).Create(ra)
}

// ApplyDeployment Will help persist a deployment
func (tu *TestUtils) ApplyDeployment(deploymentBuilder utils.DeploymentBuilder) {

	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	rd := deploymentBuilder.BuildRD()
	envNamespace := CreateEnvNamespace(tu.client, rd.GetName(), rd.Spec.Environment)
	tu.radixclient.RadixV1().RadixDeployments(envNamespace).Create(rd)
	tu.deploymentHandler.ObjectCreated(rd)
}

// CreateClusterPrerequisites Will do the needed setup which is part of radix boot
func (tu *TestUtils) CreateClusterPrerequisites() {
	tu.client.CoreV1().Secrets(corev1.NamespaceDefault).Create(&corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-docker",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"config.json": []byte("abcd"),
		},
	})

	tu.client.CoreV1().Secrets(corev1.NamespaceDefault).Create(&corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-known-hosts-git",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"known_hosts": []byte("abcd"),
		},
	})

	tu.client.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "radix-config",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"clustername": "abcd",
		},
	})
}

// CreateAppNamespace Helper method to creat app namespace
func CreateAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := getAppNamespace(appName)
	createNamespace(kubeclient, ns)
	return ns
}

// CreateEnvNamespace Helper method to creat env namespace
func CreateEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := GetNamespaceForApplicationEnvironment(appName, environment)
	createNamespace(kubeclient, ns)
	return ns
}

// GetNamespaceForApplicationEnvironment Helper method to get namespace name for app environment
func GetNamespaceForApplicationEnvironment(appName, environment string) string {
	return fmt.Sprintf("%s-%s", appName, environment)
}

func getAppNamespace(appName string) string {
	return fmt.Sprintf("%s-app", appName)
}

func createNamespace(kubeclient kubernetes.Interface, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
}
