package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	radixhttp "github.com/equinor/radix-common/net/http"
	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	mock2 "github.com/equinor/radix-operator/api-server/api/metrics/mock"
	"github.com/equinor/radix-operator/api-server/api/metrics/prometheus"
	"github.com/equinor/radix-operator/api-server/api/metrics/prometheus/mock"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	"github.com/equinor/radix-operator/api-server/api/utils"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	"github.com/equinor/radix-operator/api-server/internal/config"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	jobPipeline "github.com/equinor/radix-operator/pkg/apis/pipeline"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	builders "github.com/equinor/radix-operator/pkg/apis/utils"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/google/uuid"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/oklog/ulid/v2"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tektonclientfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	dynamicclient "sigs.k8s.io/controller-runtime/pkg/client"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName    = "AnyClusterName"
	dnsZone        = "some-dns-zone.com"
	subscriptionId = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T, options ...ApplicationHandlerOption) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, dynamicclient.Client, *secretproviderfake.Clientset, *certfake.Clientset, *tektonclientfake.Clientset) {
	return setupTestWithFactory(t, newTestApplicationHandlerFactory(
		config.Config{DNSZone: dnsZone},
		func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
			return true, nil
		},
		options...,
	))
}

func customWarningCollector(handler CollectContextWarningsFunc) ApplicationHandlerOption {
	return func(ah *ApplicationHandler) {
		ah.getWarningCollectionFromContext = handler
	}
}

func setupTestWithFactory(t *testing.T, handlerFactory ApplicationHandlerFactory) (*commontest.Utils, *controllertest.Utils, *kubefake.Clientset, *radixfake.Clientset, *kedafake.Clientset, dynamicclient.Client, *secretproviderfake.Clientset, *certfake.Clientset, *tektonclientfake.Clientset) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixclient := radixfake.NewSimpleClientset() //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	dynamicClient := commontest.CreateClient()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certfake.NewSimpleClientset()
	tektonClient := tektonclientfake.NewSimpleClientset() //nolint:staticcheck

	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, subscriptionId)
	require.NoError(t, err)
	prometheusHandlerMock := createPrometheusHandlerMock(t, nil)

	// controllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(
		kubeclient,
		radixclient,
		kedaClient,
		secretproviderclient,
		certClient,
		tektonClient,
		mockValidator,
		NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			}, handlerFactory, prometheusHandlerMock),
	)

	return &commonTestUtils, &controllerTestUtils, kubeclient, radixclient, kedaClient, dynamicClient, secretproviderclient, certClient, tektonClient
}

func createPrometheusHandlerMock(t *testing.T, mockHandler *func(handler *mock.MockQueryAPI)) *metrics.Handler {
	ctrl := gomock.NewController(t)

	promQueryApi := mock.NewMockQueryAPI(ctrl)
	promClient := prometheus.NewClient(promQueryApi)

	metricsHandler := metrics.NewHandler(promClient)
	if mockHandler != nil {
		(*mockHandler)(promQueryApi)
	} else {
		promQueryApi.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&model.Vector{}, nil, nil)
	}
	return metricsHandler
}

func TestGetApplications_HasAccessToSomeRR(t *testing.T) {
	commonTestUtils, _, kubeclient, radixclient, kedaClient, _, secretproviderclient, certClient, _ := setupTest(t)

	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-second-app.git").WithAdGroups([]string{"2"}).WithName("my-second-app"))
	require.NoError(t, err)

	t.Run("no access", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return false, nil
			}, newTestApplicationHandlerFactory(config.Config{},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("access to single app", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, rr v1.RadixRegistration) (bool, error) {
				return rr.GetName() == "my-second-app", nil
			}, newTestApplicationHandlerFactory(config.Config{},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("access to all app", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return true, nil
			}, newTestApplicationHandlerFactory(config.Config{},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 2, len(applications))
	})
}

func TestGetApplications_WithFilterOnSSHRepo_Filter(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithCloneURL("git@github.com:Equinor/my-app.git"))
	require.NoError(t, err)

	// Test
	t.Run("matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app.git")))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})

	t.Run("not matching repo", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications?sshRepo=%s", url.QueryEscape("git@github.com:Equinor/my-app2.git")))
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("no filter", func(t *testing.T) {
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications")
		response := <-responseChannel

		applications := make([]*applicationModels.ApplicationSummary, 0)
		err = controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
	})
}

func TestSearchApplicationsGet(t *testing.T) {
	// Setup
	commonTestUtils, _, kubeclient, radixclient, kedaClient, _, secretproviderclient, certClient, _ := setupTest(t)
	appNames := []string{"app-1", "app-2"}

	for _, appName := range appNames {
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName(appName))
		require.NoError(t, err)
	}

	app2Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	err := createRadixJob(commonTestUtils, appNames[1], "app-2-job-1", app2Job1Started)
	require.NoError(t, err)
	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			ARadixDeployment().
			WithAppName(appNames[1]).
			WithComponent(
				builders.
					NewDeployComponentBuilder(),
			),
	)
	require.NoError(t, err)

	prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewApplicationController(
		func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
			return true, nil
		}, newTestApplicationHandlerFactory(config.Config{},
			func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
				return true, nil
			}), prometheusHandlerMock))

	// Tests
	t.Run("search for "+appNames[0], func(t *testing.T) {
		params := "apps=" + appNames[0]
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[0], applications[0].Name)
	})

	t.Run("search for both apps", func(t *testing.T) {
		params := "apps=" + strings.Join(appNames, ",")
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 2, len(applications))
	})

	t.Run("empty appname list", func(t *testing.T) {
		params := "apps="
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'LatestJobSummary'", func(t *testing.T) {
		params := []string{"apps=" + appNames[1], "includeLatestJobSummary=true"}
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.NotNil(t, applications[0].LatestJob)
		assert.Nil(t, applications[0].Environments)
	})

	t.Run("search for "+appNames[1]+" - with includeFields 'Environments'", func(t *testing.T) {
		params := []string{"apps=" + appNames[1], "includeEnvironments=true"}
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 1, len(applications))
		assert.Equal(t, appNames[1], applications[0].Name)
		assert.Nil(t, applications[0].LatestJob)
		assert.NotNil(t, applications[0].Environments)
	})

	t.Run("search for "+appNames[0]+" - no access", func(t *testing.T) {
		prometheusHandlerMock := createPrometheusHandlerMock(t, nil)
		mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
		mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
		controllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewApplicationController(
			func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
				return false, nil
			}, newTestApplicationHandlerFactory(config.Config{},
				func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
					return true, nil
				}), prometheusHandlerMock))
		params := "apps=" + appNames[0]
		responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+params)
		response := <-responseChannel

		applications := make([]applicationModels.ApplicationSummary, 0)
		err := controllertest.GetResponseBody(response, &applications)
		require.NoError(t, err)
		assert.Equal(t, 0, len(applications))
	})
}

func TestSearchApplicationsGet_WithJobs_ShouldOnlyHaveLatest(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _, _ := setupTest(t)
	apps := []applicationModels.Application{
		{Name: "app-1", Jobs: []*jobModels.JobSummary{
			{Name: "app-1-job-1", Started: createTime("2018-11-12T11:45:26Z")},
		}},
		{Name: "app-2", Jobs: []*jobModels.JobSummary{
			{Name: "app-2-job-1", Started: createTime("2018-11-12T12:30:14Z")},
			{Name: "app-2-job-2", Started: createTime("2018-11-20T09:00:00Z")},
			{Name: "app-2-job-3", Started: createTime("2018-11-20T09:00:01Z")},
		}},
		{Name: "app-3"},
	}

	for _, app := range apps {
		commontest.CreateAppNamespace(kubeclient, app.Name)
		_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
			WithName(app.Name))
		require.NoError(t, err)

		for _, job := range app.Jobs {
			if job.Started != nil {
				err = createRadixJob(commonTestUtils, app.Name, job.Name, *job.Started)
				require.NoError(t, err)
			}
		}
	}

	// Test
	params := []string{
		"apps=" + strings.Join(slice.Reduce(apps, []string{}, func(names []string, app applicationModels.Application) []string { return append(names, app.Name) }), ","),
		"includeLatestJobSummary=true",
	}
	responseChannel := controllerTestUtils.ExecuteRequest("GET", "/api/v1/applications/_search?"+strings.Join(params, "&"))
	response := <-responseChannel

	applications := make([]*applicationModels.ApplicationSummary, 0)
	err := controllertest.GetResponseBody(response, &applications)
	require.NoError(t, err)

	for _, application := range applications {
		if app, _ := slice.FindFirst(apps, func(app applicationModels.Application) bool { return strings.EqualFold(application.Name, app.Name) }); app.Jobs != nil {
			assert.NotNil(t, application.LatestJob)
			assert.Equal(t, app.Jobs[len(app.Jobs)-1].Name, application.LatestJob.Name)
		} else {
			assert.Nil(t, application.LatestJob)
		}
	}
}

// Test warning, require acks
func TestCreateApplication_Warnings_ShouldWarn(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, customWarningCollector(func(_ context.Context) []string {
		return []string{"warning: some custom warning"}
	}))

	// Test
	parameters := buildApplicationRegistrationRequest(anApplicationRegistration().Build(), false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.Contains(t, applicationRegistrationUpsertResponse.Warnings, "warning: some custom warning")
}

// TODO: Test succeed with warnings
//
//nolint:godox
func TestCreateApplication_WithAcknowledgeWarning_ShouldSuccess(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t, customWarningCollector(func(_ context.Context) []string {
		return []string{"warning: some custom warning"}
	}))

	// Test
	parameters := buildApplicationRegistrationRequest(anApplicationRegistration().Build(), true)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Empty(t, applicationRegistrationUpsertResponse.Warnings)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
}

func TestGetApplication_AllFieldsAreSet(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	adGroups, adUsers := []string{uuid.New().String()}, []string{uuid.New().String()}
	readerAdGroups, readerAdUsers := []string{uuid.New().String()}, []string{uuid.New().String()}
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			WithSharedSecret("Any secret").
			WithAdGroups(adGroups).
			WithAdUsers(adUsers).
			WithReaderAdGroups(readerAdGroups).
			WithReaderAdUsers(readerAdUsers).
			WithConfigBranch("abranch").
			WithRadixConfigFullName("a/custom-radixconfig.yaml").
			WithConfigurationItem("ci").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err := controllertest.GetResponseBody(response, &application)
	assert.NoError(t, err)

	assert.Equal(t, "https://github.com/Equinor/any-repo", application.Registration.Repository)
	assert.Equal(t, "Any secret", application.Registration.SharedSecret)
	assert.Equal(t, adGroups, application.Registration.AdGroups)
	assert.Equal(t, adUsers, application.Registration.AdUsers)
	assert.Equal(t, readerAdGroups, application.Registration.ReaderAdGroups)
	assert.Equal(t, readerAdUsers, application.Registration.ReaderAdUsers)
	assert.Equal(t, "test-principal", application.Registration.Creator)
	assert.Equal(t, "abranch", application.Registration.ConfigBranch)
	assert.Equal(t, "a/custom-radixconfig.yaml", application.Registration.RadixConfigFullName)
	assert.Equal(t, "ci", application.Registration.ConfigurationItem)
}

func TestGetApplication_WithJobs(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
		WithName("any-name"))
	require.NoError(t, err)

	commontest.CreateAppNamespace(kubeclient, "any-name")
	app1Job1Started, _ := radixutils.ParseTimestamp("2018-11-12T11:45:26Z")
	app1Job2Started, _ := radixutils.ParseTimestamp("2018-11-12T12:30:14Z")
	app1Job3Started, _ := radixutils.ParseTimestamp("2018-11-20T09:00:00Z")

	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-1", app1Job1Started)
	require.NoError(t, err)
	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-2", app1Job2Started)
	require.NoError(t, err)
	err = createRadixJob(commonTestUtils, "any-name", "any-name-job-3", app1Job3Started)
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, 3, len(application.Jobs))
}

func TestGetApplication_BuildKitOptions(t *testing.T) {
	scenarios := map[string]struct {
		useBuildKit           *bool
		useBuildCache         *bool
		expectedUseBuildKit   bool
		expectedUseBuildCache bool
	}{
		"no buildkit or buildcache": {
			useBuildKit:           nil,
			useBuildCache:         nil,
			expectedUseBuildKit:   false,
			expectedUseBuildCache: false,
		},
		"buildkit and buildcache": {
			useBuildKit:           pointers.Ptr(true),
			useBuildCache:         pointers.Ptr(true),
			expectedUseBuildKit:   true,
			expectedUseBuildCache: true,
		},
		"buildkit only": {
			useBuildKit:           pointers.Ptr(true),
			useBuildCache:         nil,
			expectedUseBuildKit:   true,
			expectedUseBuildCache: true,
		},
		"buildcache only": {
			useBuildKit:           nil,
			useBuildCache:         pointers.Ptr(true),
			expectedUseBuildKit:   false,
			expectedUseBuildCache: false,
		},
		"buildkit and buildcache false": {
			useBuildKit:           pointers.Ptr(false),
			useBuildCache:         pointers.Ptr(false),
			expectedUseBuildKit:   false,
			expectedUseBuildCache: false,
		},
		"buildkit false and buildcache true": {
			useBuildKit:           pointers.Ptr(false),
			useBuildCache:         pointers.Ptr(true),
			expectedUseBuildKit:   false,
			expectedUseBuildCache: false,
		},
		"buildkit true and buildcache false": {
			useBuildKit:           pointers.Ptr(true),
			useBuildCache:         pointers.Ptr(false),
			expectedUseBuildKit:   true,
			expectedUseBuildCache: false,
		},
	}
	for name, ts := range scenarios {
		t.Run(name, func(t *testing.T) {
			// Setup
			commonTestUtils, controllerTestUtils, kubeclient, _, _, _, _, _, _ := setupTest(t)
			_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().
				WithName("any-name"))
			require.NoError(t, err)
			_, err = commonTestUtils.ApplyApplication(builders.ARadixApplication().
				WithAppName("any-name").WithBuildKit(ts.useBuildKit).WithBuildCache(ts.useBuildCache))
			require.NoError(t, err)

			commontest.CreateAppNamespace(kubeclient, "any-name")

			// Test
			responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
			response := <-responseChannel

			application := applicationModels.Application{}
			err = controllertest.GetResponseBody(response, &application)
			require.NoError(t, err)
			assert.Equal(t, ts.expectedUseBuildKit, application.UseBuildKit, "Invalid UseBuildKit")
			assert.Equal(t, ts.expectedUseBuildCache, application.UseBuildCache, "Invalid UseBuildCache")
		})
	}
}

func TestGetApplication_WithEnvironments(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, radix, _, _, _, _, _ := setupTest(t)

	anyAppName := "any-app"
	anyOrphanedEnvironment := "feature"

	_, err := commonTestUtils.ApplyRegistration(builders.
		NewRegistrationBuilder().
		WithName(anyAppName))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyApplication(builders.
		NewRadixApplicationBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev", "master").
		WithEnvironment("prod", "release"))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(context.Background(), builders.
		NewDeploymentBuilder().
		WithAppName(anyAppName).
		WithEnvironment("dev").
		WithImageTag("someimageindev"))
	require.NoError(t, err)

	_, err = commonTestUtils.ApplyDeployment(
		context.Background(),
		builders.
			NewDeploymentBuilder().
			WithAppName(anyAppName).
			WithEnvironment(anyOrphanedEnvironment).
			WithImageTag("someimageinfeature"))
	require.NoError(t, err)

	// Set RE statuses
	devRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "dev"), metav1.GetOptions{})
	require.NoError(t, err)
	devRe.Status.Reconciled = metav1.Now()
	_, err = radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), devRe, metav1.UpdateOptions{})
	require.NoError(t, err)
	prodRe, err := radix.RadixV1().RadixEnvironments().Get(context.Background(), builders.GetEnvironmentNamespace(anyAppName, "prod"), metav1.GetOptions{})
	require.NoError(t, err)
	prodRe.Status.Reconciled = metav1.Time{}
	_, err = radix.RadixV1().RadixEnvironments().UpdateStatus(context.Background(), prodRe, metav1.UpdateOptions{})
	require.NoError(t, err)

	orphanedRe, _ := commonTestUtils.ApplyEnvironment(builders.
		NewEnvironmentBuilder().
		WithAppLabel().
		WithAppName(anyAppName).
		WithEnvironmentName(anyOrphanedEnvironment))
	orphanedRe.Status.Reconciled = metav1.Now()
	orphanedRe.Status.Orphaned = true
	orphanedRe.Status.OrphanedTimestamp = pointers.Ptr(metav1.Now())
	_, err = radix.RadixV1().RadixEnvironments().Update(context.Background(), orphanedRe, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", anyAppName))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, 3, len(application.Environments))

	for _, environment := range application.Environments {
		if strings.EqualFold(environment.Name, "dev") {
			assert.Equal(t, environmentModels.Consistent.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, "prod") {
			assert.Equal(t, environmentModels.Pending.String(), environment.Status)
			assert.Nil(t, environment.ActiveDeployment)
		} else if strings.EqualFold(environment.Name, anyOrphanedEnvironment) {
			assert.Equal(t, environmentModels.Orphan.String(), environment.Status)
			assert.NotNil(t, environment.ActiveDeployment)
		}
	}
}

func TestUpdateApplication_MismatchingNameOrNotExists_ShouldFailAsIllegalOperation(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	parameters := buildApplicationRegistrationRequest(anApplicationRegistration().WithName("any-name").Build(), false)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("any-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-name"), errorResponse.Message)

	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("another-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, "App name any-name does not correspond with application name another-name", errorResponse.Message)

	parameters = buildApplicationRegistrationRequest(anApplicationRegistration().WithName("another-name").Build(), false)
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "another-name"), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUpdateApplication_AbleToSetAnySpecField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	builder :=
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/a-repo").
			WithSharedSecret("")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test Repository
	newRepository := "https://github.com/Equinor/any-repo"
	builder = builder.
		WithRepository(newRepository)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response := <-responseChannel

	applicationRegistrationUpsertResponse := applicationModels.ApplicationRegistrationUpsertResponse{}
	err := controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, applicationRegistrationUpsertResponse.ApplicationRegistration)
	assert.Equal(t, newRepository, applicationRegistrationUpsertResponse.ApplicationRegistration.Repository)

	// Test SharedSecret
	newSharedSecret := "Any shared secret"
	builder = builder.
		WithSharedSecret(newSharedSecret)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newSharedSecret, applicationRegistrationUpsertResponse.ApplicationRegistration.SharedSecret)

	// Test ConfigBranch
	newConfigBranch := "newcfgbranch"
	builder = builder.
		WithConfigBranch(newConfigBranch)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newConfigBranch, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigBranch)

	// Test ConfigurationItem
	newConfigurationItem := "newci"
	builder = builder.
		WithConfigurationItem(newConfigurationItem)

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s", "any-name"), buildApplicationRegistrationRequest(builder.Build(), false))
	response = <-responseChannel
	applicationRegistrationUpsertResponse = applicationModels.ApplicationRegistrationUpsertResponse{}
	err = controllertest.GetResponseBody(response, &applicationRegistrationUpsertResponse)
	require.NoError(t, err)
	assert.Equal(t, newConfigurationItem, applicationRegistrationUpsertResponse.ApplicationRegistration.ConfigurationItem)
}

func TestModifyApplication_AbleToSetField(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)
	appId := uuid.New().String()

	builder := anApplicationRegistration().
		WithName("any-name").
		WithAppID(appId).
		WithRepository("https://github.com/Equinor/a-repo").
		WithSharedSecret("").
		WithAdGroups([]string{uuid.New().String()}).
		WithAdUsers([]string{uuid.New().String()}).
		WithReaderAdGroups([]string{uuid.New().String()}).
		WithReaderAdUsers([]string{uuid.New().String()}).
		WithOwner("AN_OWNER@equinor.com").
		WithConfigBranch("main1").
		WithConfigurationItem("ci-initial")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test
	anyNewAdGroup := []string{uuid.New().String()}
	anyNewAdUser := []string{uuid.New().String()}
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			AdGroups: &anyNewAdGroup,
			AdUsers:  &anyNewAdUser,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	application := applicationModels.Application{}
	err := controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, anyNewAdUser, application.Registration.AdUsers)
	assert.Equal(t, "AN_OWNER@equinor.com", application.Registration.Owner)
	assert.Equal(t, "main1", application.Registration.ConfigBranch)
	assert.Equal(t, "ci-initial", application.Registration.ConfigurationItem)
	assert.NotEqual(t, appId, application.Registration.AppID, "AppID should be generated and not changeable")
	assert.NotEqual(t, ulid.Zero.String(), application.Registration.AppID, "AppID should be generated and not zero")

	// Test
	anyNewReaderAdGroup := []string{uuid.New().String()}
	anyNewReaderAdUser := []string{uuid.New().String()}
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ReaderAdGroups: &anyNewReaderAdGroup,
			ReaderAdUsers:  &anyNewReaderAdUser,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewReaderAdGroup, application.Registration.ReaderAdGroups)
	assert.Equal(t, anyNewReaderAdUser, application.Registration.ReaderAdUsers)

	// Test
	anyNewOwner := "A_NEW_OWNER@equinor.com"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			Owner: &anyNewOwner,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewAdGroup, application.Registration.AdGroups)
	assert.Equal(t, anyNewOwner, application.Registration.Owner)

	// Test ConfigBranch
	anyNewConfigBranch := "main2"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigBranch: &anyNewConfigBranch,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewConfigBranch, application.Registration.ConfigBranch)

	// Test ConfigurationItem
	anyNewConfigurationItem := "ci-patch"
	patchRequest = applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigurationItem: &anyNewConfigurationItem,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel

	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewConfigurationItem, application.Registration.ConfigurationItem)
}

func TestModifyApplication_AbleToUpdateRepository(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	builder := anApplicationRegistration().
		WithName("any-name").
		WithRepository("https://github.com/Equinor/a-repo")
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", buildApplicationRegistrationRequest(builder.Build(), false))
	<-responseChannel

	// Test
	anyNewRepo := "https://github.com/repo/updated-version"
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			Repository: &anyNewRepo,
		},
	}

	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	<-responseChannel

	responseChannel = controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err := controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)
	assert.Equal(t, anyNewRepo, application.Registration.Repository)
}

func TestModifyApplication_IgnoreRequireADGroupValidationWhenRequiredButCurrentIsEmpty(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, radixClient, _, _, _, _, _ := setupTest(t)

	rr, err := anApplicationRegistration().
		WithName("any-name").
		WithAdGroups(nil).
		BuildRR()
	require.NoError(t, err)
	_, err = radixClient.RadixV1().RadixRegistrations().Create(context.Background(), rr, metav1.CreateOptions{})
	require.NoError(t, err)

	// Test
	patchRequest := applicationModels.ApplicationRegistrationPatchRequest{
		ApplicationRegistrationPatch: &applicationModels.ApplicationRegistrationPatch{
			ConfigBranch: radixutils.StringPtr("dummyupdate"),
		},
	}

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PATCH", fmt.Sprintf("/api/v1/applications/%s", "any-name"), patchRequest)
	response := <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestHandleTriggerPipeline_ForNonMappedAndMappedAndMagicBranchEnvironment_JobIsNotCreatedForUnmapped(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)
	anyAppName := "any-app"
	configBranch := "magic"

	rr := builders.ARadixRegistration().WithConfigBranch(configBranch).WithAdGroups([]string{"adminGroup"})
	_, err := commonTestUtils.ApplyApplication(builders.
		ARadixApplication().
		WithRadixRegistration(rr).
		WithAppName(anyAppName).
		WithEnvironment("dev", "dev").
		WithEnvironment("prod", "release"),
	)
	require.NoError(t, err)

	// Test
	unmappedBranch := "feature"

	parameters := applicationModels.PipelineParametersBuild{Branch: unmappedBranch}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	expectedError := applicationModels.UnmatchedBranchToEnvironment(unmappedBranch)
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	// Mapped branch should start job
	parameters = applicationModels.PipelineParametersBuild{Branch: "dev"}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)

	// Magic branch should start job, even if it is not mapped
	parameters = applicationModels.PipelineParametersBuild{Branch: configBranch}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", anyAppName, v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestHandleTriggerPipeline_ExistingAndNonExistingApplication_JobIsCreatedForExisting(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	registerAppParam := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-app").
			WithConfigBranch("maincfg").
			Build(),
		false,
	)
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
	<-responseChannel

	// Test
	const pushCommitID = "4faca8595c5283a9d0f17a623b9255a0d9866a2e"

	parameters := applicationModels.PipelineParametersBuild{Branch: "master", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "another-app", v1.BuildDeploy), parameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, controllertest.AppNotFoundErrorMsg("another-app"), errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel

	assert.Equal(t, http.StatusBadRequest, response.Code)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	expectedError := applicationModels.AppNameAndBranchAreRequiredForStartingPipeline()
	assert.Equal(t, (expectedError.(*radixhttp.Error)).Message, errorResponse.Message)

	parameters = applicationModels.PipelineParametersBuild{Branch: "maincfg", CommitID: pushCommitID}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", "any-app", v1.BuildDeploy), parameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	jobSummary := jobModels.JobSummary{}
	err := controllertest.GetResponseBody(response, &jobSummary)
	require.NoError(t, err)
	assert.Equal(t, "any-app", jobSummary.AppName)
	assert.Equal(t, "maincfg", jobSummary.GitRef)
	assert.Equal(t, "branch", jobSummary.GitRefType)
	assert.Equal(t, pushCommitID, jobSummary.CommitID)
}

func TestHandleTriggerPipeline_Deploy_JobHasCorrectParameters(t *testing.T) {
	appName := "an-app"

	type scenario struct {
		name                  string
		params                applicationModels.PipelineParametersDeploy
		expectedToEnvironment string
		expectedImageTagNames map[string]string
	}

	scenarios := []scenario{
		{
			name:                  "only target environment",
			params:                applicationModels.PipelineParametersDeploy{ToEnvironment: "target"},
			expectedToEnvironment: "target",
		},
		{
			name:                  "target environment with image tags",
			params:                applicationModels.PipelineParametersDeploy{ToEnvironment: "target", ImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"}},
			expectedToEnvironment: "target",
			expectedImageTagNames: map[string]string{"component1": "tag1", "component2": "tag22"},
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			_, controllerTestUtils, _, radixclient, _, _, _, _, _ := setupTest(t)
			registerAppParam := buildApplicationRegistrationRequest(anApplicationRegistration().WithName(appName).Build(), false)
			<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Deploy), ts.params)
			<-responseChannel

			appNamespace := fmt.Sprintf("%s-app", appName)
			jobs, _ := getJobsInNamespace(radixclient, appNamespace)

			assert.Equal(t, ts.expectedToEnvironment, jobs[0].Spec.Deploy.ToEnvironment)
			assert.Equal(t, ts.expectedImageTagNames, jobs[0].Spec.Deploy.ImageTagNames)
		})
	}
}

func TestHandleTriggerPipeline_Promote_JobHasCorrectParameters(t *testing.T) {

	const (
		appName         = "an-app"
		commitId        = "475f241c-478b-49da-adfb-3c336aaab8d2"
		fromEnvironment = "origin"
		toEnvironment   = "target"
	)

	type scenario struct {
		name                      string
		existingDeploymentName    string
		requestedDeploymentName   string
		expectedDeploymentName    string
		expectedResponseBodyError string
		expectedResponseCode      int
	}
	scenarios := []scenario{
		{
			name:                    "existing full deployment name",
			existingDeploymentName:  "abc-deployment",
			requestedDeploymentName: "abc-deployment",
			expectedDeploymentName:  "abc-deployment",
			expectedResponseCode:    200,
		},
		{
			name:                    "existing short deployment name",
			existingDeploymentName:  "abc-deployment",
			requestedDeploymentName: "deployment",
			expectedDeploymentName:  "abc-deployment",
			expectedResponseCode:    200,
		},
		{
			name:                      "non existing short deployment name",
			existingDeploymentName:    "abc-deployment",
			requestedDeploymentName:   "other-name",
			expectedDeploymentName:    "",
			expectedResponseBodyError: "invalid or not existing deployment name",
			expectedResponseCode:      400,
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			commonTestUtils, controllerTestUtils, _, radixclient, _, _, _, _, _ := setupTest(t)
			_, err := commonTestUtils.ApplyDeployment(
				context.Background(),
				builders.
					ARadixDeployment().
					WithAppName(appName).
					WithDeploymentName(ts.existingDeploymentName).
					WithEnvironment(fromEnvironment).
					WithLabel(kube.RadixCommitLabel, commitId).
					WithCondition(v1.DeploymentInactive))
			require.NoError(t, err)

			parameters := applicationModels.PipelineParametersPromote{
				FromEnvironment: fromEnvironment,
				ToEnvironment:   toEnvironment,
				DeploymentName:  ts.requestedDeploymentName,
			}

			registerAppParam := buildApplicationRegistrationRequest(anApplicationRegistration().WithName(appName).Build(), false)
			<-controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", registerAppParam)
			responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/pipelines/%s", appName, v1.Promote), parameters)
			response := <-responseChannel
			assert.Equal(t, ts.expectedResponseCode, response.Code)
			if ts.expectedResponseCode != 200 {
				assert.NotNil(t, response.Body, "Empty respond body")
				type RespondBody struct {
					Type    string `json:"type"`
					Message string `json:"message"`
					Error   string `json:"error"`
				}
				body := RespondBody{}
				err = json.Unmarshal(response.Body.Bytes(), &body)
				require.NoError(t, err)
				require.Equal(t, ts.expectedResponseBodyError, body.Error, "invalid respond error")

			} else {
				appNamespace := fmt.Sprintf("%s-app", appName)
				jobs, err := getJobsInNamespace(radixclient, appNamespace)
				require.NoError(t, err)
				require.Len(t, jobs, 1)
				assert.Equal(t, jobs[0].Spec.Promote.FromEnvironment, fromEnvironment)
				assert.Equal(t, jobs[0].Spec.Promote.ToEnvironment, toEnvironment)
				assert.Equal(t, ts.expectedDeploymentName, jobs[0].Spec.Promote.DeploymentName)
				assert.Equal(t, jobs[0].Spec.Promote.CommitID, commitId)
			}
		})
	}
}

func TestDeleteApplication_ApplicationIsDeleted(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			Build(),
		false,
	)

	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-responseChannel

	// Test
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-non-existing"))
	response := <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)

	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)

	// Application should no longer exist
	responseChannel = controllerTestUtils.ExecuteRequest("DELETE", fmt.Sprintf("/api/v1/applications/%s", "any-name"))
	response = <-responseChannel
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestGetApplication_WithAppAlias_ContainsAppAlias(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, client, radixclient, kedaClient, dynamicClient, secretproviderclient, certClient, _ := setupTest(t)
	err := utils.ApplyDeploymentWithSync(client, radixclient, kedaClient, dynamicClient, commonTestUtils, secretproviderclient, certClient, builders.ARadixDeployment().
		WithRadixApplication(builders.ARadixApplication().
			WithAppName("any-app").
			WithDNSAppAlias("prod", "frontend")).
		WithAppName("any-app").
		WithEnvironment("prod").
		WithComponents(
			builders.NewDeployComponentBuilder().
				WithName("frontend").
				WithPort("http", 8080).
				WithPublicPort("http").
				WithDNSAppAlias(true),
			builders.NewDeployComponentBuilder().
				WithName("backend")))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s", "any-app"))
	response := <-responseChannel

	application := applicationModels.Application{}
	err = controllertest.GetResponseBody(response, &application)
	require.NoError(t, err)

	require.NotNil(t, application.AppAlias)
	assert.Equal(t, "frontend", application.AppAlias.ComponentName)
	assert.Equal(t, "prod", application.AppAlias.EnvironmentName)
	assert.Equal(t, fmt.Sprintf("%s.app.%s", "any-app", dnsZone), application.AppAlias.URL)
}

func TestListPipeline_ReturnsAvailablePipelines(t *testing.T) {
	supportedPipelines := jobPipeline.GetSupportedPipelines()

	// Setup
	commonTestUtils, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)
	_, err := commonTestUtils.ApplyRegistration(builders.ARadixRegistration().WithName("some-app"))
	require.NoError(t, err)

	// Test
	responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/pipelines", "some-app"))
	response := <-responseChannel

	pipelines := make([]string, 0)
	err = controllertest.GetResponseBody(response, &pipelines)
	require.NoError(t, err)
	assert.Equal(t, len(supportedPipelines), len(pipelines))
}

func TestRegenerateDeployKey_WhenApplicationNotExist_Fail(t *testing.T) {
	// Setup
	_, controllerTestUtils, _, _, _, _, _, _, _ := setupTest(t)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	regenerateParameters := &applicationModels.RegenerateSharedSecretData{SharedSecret: "new shared secret"}
	appName := "any-non-existing-name"
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-shared-secret", appName), regenerateParameters)
	response := <-responseChannel

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestRegenerateDeployKey_UpdatesSharedSecret(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _, _ := setupTest(t)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	const newSharedSecret = "new shared secret"
	regenerateParameters := &applicationModels.RegenerateSharedSecretData{SharedSecret: newSharedSecret}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-shared-secret", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	radixRegistration, err := radixClient.RadixV1().RadixRegistrations().Get(context.Background(), appName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, newSharedSecret, radixRegistration.Spec.SharedSecret)

	deployKeyAndSecret := &applicationModels.DeployKeyAndSecret{}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("GET", fmt.Sprintf("/api/v1/applications/%s/deploy-key-and-secret", appName), regenerateParameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	err = controllertest.GetResponseBody(response, &deployKeyAndSecret)
	require.NoError(t, err)
	assert.Equal(t, newSharedSecret, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_CreateSharedSecret(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _, _ := setupTest(t)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Test
	parameters := buildApplicationRegistrationRequest(
		anApplicationRegistration().
			WithName("any-name").
			WithRepository("https://github.com/Equinor/any-repo").
			Build(),
		false,
	)

	appResponseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", "/api/v1/applications", parameters)
	<-appResponseChannel

	regenerateParameters := &applicationModels.RegenerateSharedSecretData{}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-shared-secret", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	radixRegistration, err := radixClient.RadixV1().RadixRegistrations().Get(context.Background(), appName, metav1.GetOptions{})
	require.NoError(t, err)
	newSharedSecret := radixRegistration.Spec.SharedSecret
	assert.NotEmpty(t, newSharedSecret)

	deployKeyAndSecret := &applicationModels.DeployKeyAndSecret{}
	responseChannel = controllerTestUtils.ExecuteRequestWithParameters("GET", fmt.Sprintf("/api/v1/applications/%s/deploy-key-and-secret", appName), regenerateParameters)
	response = <-responseChannel
	assert.Equal(t, http.StatusOK, response.Code)
	err = controllertest.GetResponseBody(response, &deployKeyAndSecret)
	require.NoError(t, err)
	assert.Equal(t, newSharedSecret, deployKeyAndSecret.SharedSecret)
}

func TestRegenerateDeployKey_NoSecretInParam_SecretIsReCreated(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _, _ := setupTest(t)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been created
	firstSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(firstSecret.Data[defaults.GitPrivateKeySecretKey]), 1)

	// calling regenerate-deploy-key in order to delete secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyData{}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	err = utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been re-created and is different from first secret
	secondSecret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(secondSecret.Data[defaults.GitPrivateKeySecretKey]), 1)
	assert.NotEqual(t, firstSecret.Data[defaults.GitPrivateKeySecretKey], secondSecret.Data[defaults.GitPrivateKeySecretKey])
}

func TestRegenerateDeployKey_PrivateKeyInParam_SavedPrivateKeyIsEqualToWebParam(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _, _ := setupTest(t)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// make some valid private key
	deployKey, err := builders.GenerateDeployKey()
	assert.NoError(t, err)

	// calling regenerate-deploy-key in order to set secret
	regenerateParameters := &applicationModels.RegenerateDeployKeyData{PrivateKey: deployKey.PrivateKey}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusNoContent, response.Code)

	// forcing resync of RR
	err = utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// Check that secret has been re-created and is equal to the one in the web parameter
	secret, err := kubeUtil.CoreV1().Secrets(builders.GetAppNamespace(appName)).Get(context.Background(), defaults.GitPrivateKeySecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, deployKey.PrivateKey, string(secret.Data[defaults.GitPrivateKeySecretKey]))
}

func TestRegenerateDeployKey_InvalidKeyInParam_ErrorIsReturned(t *testing.T) {
	// Setup
	commonTestUtils, controllerTestUtils, kubeUtil, radixClient, kedaClient, _, _, _, _ := setupTest(t)
	appName := "any-name"
	rrBuilder := builders.ARadixRegistration().WithName(appName).WithCloneURL("git@github.com:Equinor/my-app.git")

	// Creating RR and syncing it
	err := utils.ApplyRegistrationWithSync(kubeUtil, radixClient, kedaClient, commonTestUtils, rrBuilder)
	require.NoError(t, err)

	// calling regenerate-deploy-key with invalid private key, expecting error
	regenerateParameters := &applicationModels.RegenerateDeployKeyData{PrivateKey: "invalid key"}
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("POST", fmt.Sprintf("/api/v1/applications/%s/regenerate-deploy-key", appName), regenerateParameters)
	response := <-responseChannel
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func Test_GetUsedResources(t *testing.T) {
	type scenario struct {
		name                       string
		expectedError              error
		queryString                string
		expectedUsedResourcesError error
	}

	scenarios := []scenario{
		{
			name: "Get used resources",
		},
		{
			name:        "Get used resources with arguments",
			queryString: "?environment=prod&component=component1&duration=10d&since=2w",
		},
		{
			name:                       "UsedResources returns an error",
			expectedUsedResourcesError: errors.New("error-123"),
			expectedError:              errors.New("error: error-123"),
		},
	}

	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			_, _, kubeClient, radixClient, kedaClient, _, secretProviderClient, certClient, _ := setupTest(t)

			_, err := radixClient.RadixV1().RadixRegistrations().Create(context.Background(), builders.ARadixRegistration().BuildRR(), metav1.CreateOptions{})
			require.NoError(t, err)

			ra := builders.ARadixApplication().BuildRA()
			_, err = radixClient.RadixV1().RadixApplications(builders.GetAppNamespace(ra.GetName())).Create(context.Background(), ra, metav1.CreateOptions{})
			require.NoError(t, err)

			expectedUtilization := applicationModels.NewPodResourcesUtilizationResponse()
			expectedUtilization.SetCpuRequests("test", "app", "app-abcd-1", 1)

			cpuReqs := []metrics.LabeledResults{{Value: 1, Environment: "test", Component: "app", Pod: "app-abcd-1"}}

			validator := authnmock.NewMockValidatorInterface(ctrl)
			validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(controllertest.NewTestPrincipal(true), nil)

			client := mock2.NewMockClient(ctrl)
			client.EXPECT().GetCpuRequests(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(cpuReqs, nil)
			client.EXPECT().GetCpuAverage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemoryRequests(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, nil)
			client.EXPECT().GetMemoryMaximum(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return([]metrics.LabeledResults{}, ts.expectedError)
			metricsHandler := metrics.NewHandler(client)

			controllerTestUtils := controllertest.NewTestUtils(kubeClient, radixClient, kedaClient, secretProviderClient, certClient, nil, validator, NewApplicationController(
				func(_ context.Context, _ kubernetes.Interface, _ v1.RadixRegistration) (bool, error) {
					return true, nil
				},
				newTestApplicationHandlerFactory(
					config.Config{},
					func(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapName string) (bool, error) {
						return true, nil
					},
				),
				metricsHandler,
			))

			responseChannel := controllerTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/utilization", ra.GetName()))
			response := <-responseChannel
			if ts.expectedError != nil {
				assert.Equal(t, http.StatusBadRequest, response.Code)
				errorResponse, _ := controllertest.GetErrorResponse(response)
				assert.Equal(t, ts.expectedError.Error(), errorResponse.Error())
				return
			}
			assert.Equal(t, http.StatusOK, response.Code)
			actualUtilization := &applicationModels.ReplicaResourcesUtilizationResponse{}
			err = controllertest.GetResponseBody(response, &actualUtilization)
			require.NoError(t, err)
			assert.Equal(t, expectedUtilization, actualUtilization)
		})
	}
}

func createRadixJob(commonTestUtils *commontest.Utils, appName, jobName string, started time.Time) error {
	_, err := commonTestUtils.ApplyJob(
		builders.ARadixBuildDeployJob().
			WithAppName(appName).
			WithJobName(jobName).
			WithStatus(builders.NewJobStatusBuilder().
				WithCondition(v1.JobSucceeded).
				WithStarted(started.UTC()).
				WithSteps(
					builders.ACloneConfigStep().
						WithCondition(v1.JobSucceeded).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))),
					builders.ARadixPipelineStep().
						WithCondition(v1.JobRunning).
						WithStarted(started.UTC()).
						WithEnded(started.Add(time.Second*time.Duration(100))))))
	return err
}

func getJobsInNamespace(radixclient *radixfake.Clientset, appNamespace string) ([]v1.RadixJob, error) {
	jobs, err := radixclient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return jobs.Items, nil
}

// anApplicationRegistration Constructor for application builder with test values
func anApplicationRegistration() applicationModels.ApplicationRegistrationBuilder {
	return applicationModels.NewApplicationRegistrationBuilder().
		WithName("my-app").
		WithRepository("https://github.com/Equinor/my-app").
		WithSharedSecret("AnySharedSecret").
		WithAdGroups([]string{"a6a3b81b-34gd-sfsf-saf2-7986371ea35f"}).
		WithReaderAdGroups([]string{"40e794dc-244c-4d0a-9f29-55fda1fe3972"}).
		WithCreator("a_test_user@equinor.com").
		WithConfigurationItem("2b0781a7db131784551ea1ea4b9619c9").
		WithConfigBranch("main")
}

func buildApplicationRegistrationRequest(applicationRegistration applicationModels.ApplicationRegistration, acknowledgeWarnings bool) *applicationModels.ApplicationRegistrationRequest {
	return &applicationModels.ApplicationRegistrationRequest{
		ApplicationRegistration: &applicationRegistration,
		AcknowledgeWarnings:     acknowledgeWarnings,
	}
}

type testApplicationHandlerFactory struct {
	config                  config.Config
	hasAccessToGetConfigMap hasAccessToGetConfigMapFunc
	options                 []ApplicationHandlerOption
}

func newTestApplicationHandlerFactory(config config.Config, hasAccessToGetConfigMap hasAccessToGetConfigMapFunc, options ...ApplicationHandlerOption) *testApplicationHandlerFactory {
	return &testApplicationHandlerFactory{
		config:                  config,
		hasAccessToGetConfigMap: hasAccessToGetConfigMap,
		options:                 options,
	}
}

// Create creates a new ApplicationHandler
func (f *testApplicationHandlerFactory) Create(accounts models.Accounts) ApplicationHandler {
	return NewApplicationHandler(accounts, f.config, f.hasAccessToGetConfigMap, f.options...)
}

func createTime(timestamp string) *time.Time {
	if timestamp == "" {
		return &time.Time{}
	}

	t, _ := time.Parse(time.RFC3339, timestamp)
	return &t
}
