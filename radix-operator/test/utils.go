package test

import (
	test "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO: Merge this with "github.com/equinor/radix-operator/pkg/apis/test" once the handler functionality is in the pkg structure

// Utils Instance variables
type Utils struct {
	client              kubernetes.Interface
	radixclient         radixclient.Interface
	registrationHandler common.Handler
	applicationHandler  common.Handler
	deploymentHandler   common.Handler
}

// NewHandlerTestUtils Constructor
func NewHandlerTestUtils(client kubernetes.Interface, radixclient radixclient.Interface, registrationHandler common.Handler, applicationHandler common.Handler, deploymentHandler common.Handler) Utils {
	return Utils{
		client:              client,
		radixclient:         radixclient,
		registrationHandler: registrationHandler,
		applicationHandler:  applicationHandler,
		deploymentHandler:   deploymentHandler,
	}
}

// ApplyRegistration Will help persist an application registration
func (tu *Utils) ApplyRegistration(registrationBuilder utils.RegistrationBuilder) error {
	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyRegistration(registrationBuilder)

	rr := registrationBuilder.BuildRR()
	err = tu.registrationHandler.ObjectCreated(rr)
	if err != nil {
		return err
	}

	return nil
}

// ApplyApplication Will help persist an application
func (tu *Utils) ApplyApplication(applicationBuilder utils.ApplicationBuilder) error {
	if applicationBuilder.GetRegistrationBuilder() != nil {
		tu.ApplyRegistration(applicationBuilder.GetRegistrationBuilder())
	}

	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyApplication(applicationBuilder)
	if err != nil {
		return err
	}

	err = tu.applicationHandler.ObjectCreated(applicationBuilder.BuildRA())
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeployment Will help persist a deployment
func (tu *Utils) ApplyDeployment(deploymentBuilder utils.DeploymentBuilder) error {

	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return err
	}

	rd := deploymentBuilder.BuildRD()
	err = tu.deploymentHandler.ObjectCreated(rd)
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdate(deploymentBuilder utils.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()
	envNamespace := utils.GetEnvironmentNamespace(rd.Spec.AppName, rd.Spec.Environment)
	previousVersion, _ := tu.radixclient.RadixV1().RadixDeployments(envNamespace).Get(rd.GetName(), metav1.GetOptions{})

	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	err = tu.deploymentHandler.ObjectUpdated(previousVersion, rd)
	if err != nil {
		return err
	}

	return nil
}

// CreateClusterPrerequisites Creates needed pre-required resources in cluster
func (tu *Utils) CreateClusterPrerequisites(clustername, containerRegistry string) {
	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	testUtils.CreateClusterPrerequisites(clustername, containerRegistry)
}
