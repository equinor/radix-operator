package test

import (
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	test "github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/radix-operator/common"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

// TODO: Merge this with "github.com/equinor/radix-operator/pkg/apis/test" once the handler functionality is in the pkg structure

// Utils Instance variables
type Utils struct {
	client              kubernetes.Interface
	radixclient         radixclient.Interface
	registrationHandler common.Handler
	applicationHandler  common.Handler
	deploymentHandler   common.Handler
	eventRecoder        record.EventRecorder
}

// NewHandlerTestUtils Constructor
func NewHandlerTestUtils(client kubernetes.Interface, radixclient radixclient.Interface, registrationHandler common.Handler, applicationHandler common.Handler, deploymentHandler common.Handler) Utils {
	eventRecorder := &record.FakeRecorder{}

	return Utils{
		client:              client,
		radixclient:         radixclient,
		registrationHandler: registrationHandler,
		applicationHandler:  applicationHandler,
		deploymentHandler:   deploymentHandler,
		eventRecoder:        eventRecorder,
	}
}

// ApplyRegistration Will help persist an application registration
func (tu *Utils) ApplyRegistration(registrationBuilder utils.RegistrationBuilder) error {
	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyRegistration(registrationBuilder)

	rr := registrationBuilder.BuildRR()
	err = tu.registrationHandler.Sync(rr.Namespace, rr.Name, tu.eventRecoder)
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

	ra := applicationBuilder.BuildRA()
	err = tu.applicationHandler.Sync(ra.Namespace, ra.Name, tu.eventRecoder)
	if err != nil {
		return err
	}

	return nil
}

// ApplyDeployment Will help persist a deployment
func (tu *Utils) ApplyDeployment(deploymentBuilder utils.DeploymentBuilder) (*v1.RadixDeployment, error) {

	if deploymentBuilder.GetApplicationBuilder() != nil {
		tu.ApplyApplication(deploymentBuilder.GetApplicationBuilder())
	}

	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	rd, err := testUtils.ApplyDeployment(deploymentBuilder)
	if err != nil {
		return nil, err
	}

	err = tu.deploymentHandler.Sync(rd.Namespace, rd.Name, tu.eventRecoder)
	if err != nil {
		return nil, err
	}

	return rd, nil
}

// ApplyDeploymentUpdate Will help update a deployment
func (tu *Utils) ApplyDeploymentUpdate(deploymentBuilder utils.DeploymentBuilder) error {
	rd := deploymentBuilder.BuildRD()

	testUtils := test.NewTestUtils(tu.client, tu.radixclient)
	err := testUtils.ApplyDeploymentUpdate(deploymentBuilder)
	if err != nil {
		return err
	}

	err = tu.deploymentHandler.Sync(rd.Namespace, rd.Name, tu.eventRecoder)
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
