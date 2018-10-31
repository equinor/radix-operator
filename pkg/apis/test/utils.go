package test

import (
	log "github.com/Sirupsen/logrus"
	builders "github.com/statoil/radix-operator/pkg/apis/utils"
	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type utils struct {
	client      kubernetes.Interface
	radixclient radixclient.Interface
}

// NewTestUtils Constructor
func NewTestUtils(client kubernetes.Interface, radixclient radixclient.Interface) utils {
	return utils{
		client:      client,
		radixclient: radixclient,
	}
}

// ApplyRegistration Will help persist an application registration
func (tu *utils) ApplyRegistration(registrationBuilder builders.RegistrationBuilder) error {
	rr := registrationBuilder.BuildRR()

	_, err := tu.radixclient.RadixV1().RadixRegistrations(corev1.NamespaceDefault).Create(rr)
	if err != nil {
		return err
	}

	return nil
}

// ApplyApplication Will help persist an application
func (tu *utils) ApplyApplication(applicationBuilder builders.ApplicationBuilder) error {
	if applicationBuilder.GetRegistrationBuilder() != nil {
		tu.ApplyRegistration(applicationBuilder.GetRegistrationBuilder())
	}

	ra := applicationBuilder.BuildRA()
	appNamespace := CreateAppNamespace(tu.client, ra.GetName())
	_, err := tu.radixclient.RadixV1().RadixApplications(appNamespace).Create(ra)
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeployment Will help persist a deployment
func (tu *utils) ApplyDeployment(deploymentBuilder builders.DeploymentBuilder) error {

	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	rd := deploymentBuilder.BuildRD()
	log.Infof("%s", rd.GetObjectMeta().GetCreationTimestamp())

	envNamespace := CreateEnvNamespace(tu.client, rd.Spec.AppName, rd.Spec.Environment)
	_, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Create(rd)
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *utils) ApplyDeploymentUpdate(deploymentBuilder builders.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()
	envNamespace := builders.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)

	_, err := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Update(rd)
	if err != nil {
		return err
	}

	return nil
}

// CreateClusterPrerequisites Will do the needed setup which is part of radix boot
func (tu *utils) CreateClusterPrerequisites(clustername string) {
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
			"clustername": clustername,
		},
	})
}

// CreateAppNamespace Helper method to creat app namespace
func CreateAppNamespace(kubeclient kubernetes.Interface, appName string) string {
	ns := builders.GetAppNamespace(appName)
	createNamespace(kubeclient, ns)
	return ns
}

// CreateEnvNamespace Helper method to creat env namespace
func CreateEnvNamespace(kubeclient kubernetes.Interface, appName, environment string) string {
	ns := builders.GetEnvironmentNamespace(appName, environment)
	createNamespace(kubeclient, ns)
	return ns
}

func createNamespace(kubeclient kubernetes.Interface, ns string) {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	kubeclient.CoreV1().Namespaces().Create(&namespace)
}
