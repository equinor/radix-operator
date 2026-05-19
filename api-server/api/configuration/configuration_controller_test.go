package configuration_test

import (
	"testing"

	"github.com/equinor/radix-operator/api-server/api/configuration"
	configurationModels "github.com/equinor/radix-operator/api-server/api/configuration/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	"github.com/equinor/radix-operator/api-server/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetSettings_Authenticated(t *testing.T) {

	cfg := config.Config{
		DNSZone:            "example.com",
		ClusterName:        "test-cluster",
		ClusterEgressIps:   []string{"1.2.3.4"},
		ClusterOidcIssuers: []string{"https://issuer.example.com"},
	}

	// Setup
	controllerTestUtils := setupTest(t, cfg, true)
	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/configuration")
	response := <-responseChannel

	settings := configurationModels.ClusterConfiguration{}
	err := controllertest.GetResponseBody(response, &settings)
	require.NoError(t, err)

	assert.Equal(t, cfg.DNSZone, settings.DNSZone)
	assert.Equal(t, cfg.ClusterName, settings.ClusterName)
	assert.Equal(t, cfg.ClusterEgressIps, settings.ClusterEgressIps)
	assert.Equal(t, cfg.ClusterOidcIssuers, settings.ClusterOidcIssuers)
}

func TestGetSettings_NotAuthenticated(t *testing.T) {

	cfg := config.Config{}

	// Setup
	controllerTestUtils := setupTest(t, cfg, false)
	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/configuration")
	response := <-responseChannel

	assert.Equal(t, 403, response.Code)
}

func setupTest(t *testing.T, cfg config.Config, authenticated bool) *controllertest.Utils {
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(authenticated), nil)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	configurationControllerTestUtils := controllertest.NewTestUtils(nil, nil, nil, nil, nil, nil, mockValidator, configuration.NewConfigurationController(configuration.Init(cfg)))

	return &configurationControllerTestUtils
}
