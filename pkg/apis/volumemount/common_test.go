package volumemount

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type volumeMountTestScenario struct {
	name               string
	radixVolumeMount   radixv1.RadixVolumeMount
	expectedVolumeName string
	expectedError      string
	expectedPvcName    string
}

type testSuite struct {
	suite.Suite
	kubeClient kubernetes.Interface
	kubeUtil   *kube.Kube
}

func (s *testSuite) SetupTest() {
	s.setupTest()
}

func (s *testSuite) SetupSubTest() {
	s.setupTest()
}

func (s *testSuite) setupTest() {
	kubeClient := fake.NewSimpleClientset()
	radixClient := radixfake.NewSimpleClientset()
	s.kubeClient = kubeClient
	s.kubeUtil, _ = kube.New(kubeClient, radixClient, kedafake.NewSimpleClientset(), secretproviderfake.NewSimpleClientset())
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
