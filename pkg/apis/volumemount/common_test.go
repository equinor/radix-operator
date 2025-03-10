package volumemount

import (
	"reflect"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

// radixCommonDeployComponentFactory defines a common component factory
type radixCommonDeployComponentFactory interface {
	Create() radixv1.RadixCommonDeployComponent
	GetTargetType() reflect.Type
}

type radixDeployComponentFactory struct{}
type radixDeployJobComponentFactory struct{}

func (factory radixDeployComponentFactory) Create() radixv1.RadixCommonDeployComponent {
	return &radixv1.RadixDeployComponent{}
}

func (factory radixDeployComponentFactory) GetTargetType() reflect.Type {
	return reflect.TypeOf(&radixv1.RadixDeployComponent{}).Elem()
}

func (factory radixDeployJobComponentFactory) Create() radixv1.RadixCommonDeployComponent {
	return &radixv1.RadixDeployJobComponent{}
}

func (factory radixDeployJobComponentFactory) GetTargetType() reflect.Type {
	return reflect.TypeOf(&radixv1.RadixDeployJobComponent{}).Elem()
}

type testSuite struct {
	suite.Suite
	radixCommonDeployComponentFactories []radixCommonDeployComponentFactory
}

type TestEnv struct {
	kubeClient           kubernetes.Interface
	radixClient          versioned.Interface
	secretProviderClient secretsstorevclient.Interface
	prometheusClient     prometheusclient.Interface
	kubeUtil             *kube.Kube
	kedaClient           kedav2.Interface
}

type volumeMountTestScenario struct {
	name               string
	radixVolumeMount   radixv1.RadixVolumeMount
	expectedVolumeName string
	expectedError      string
	expectedPvcName    string
}

func (s *testSuite) SetupSuite() {
	s.radixCommonDeployComponentFactories = []radixCommonDeployComponentFactory{
		radixDeployComponentFactory{},
		radixDeployJobComponentFactory{},
	}
}

func getTestEnv() TestEnv {
	testEnv := TestEnv{
		kubeClient:           fake.NewSimpleClientset(),
		radixClient:          radixfake.NewSimpleClientset(),
		kedaClient:           kedafake.NewSimpleClientset(),
		secretProviderClient: secretproviderfake.NewSimpleClientset(),
		prometheusClient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeClient, testEnv.radixClient, testEnv.kedaClient, testEnv.secretProviderClient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

func buildRd(appName string, environment string, componentName string, identityAzureClientId string, radixVolumeMounts []radixv1.RadixVolumeMount) *radixv1.RadixDeployment {
	return operatorUtils.ARadixDeployment().
		WithAppName(appName).
		WithEnvironment(environment).
		WithComponents(operatorUtils.NewDeployComponentBuilder().
			WithName(componentName).
			WithIdentity(&radixv1.Identity{Azure: &radixv1.AzureIdentity{ClientId: identityAzureClientId}}).
			WithVolumeMounts(radixVolumeMounts...)).
		BuildRD()
}
