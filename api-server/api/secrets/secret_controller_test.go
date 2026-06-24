package secrets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	secretModels "github.com/equinor/radix-operator/api-server/api/secrets/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	"github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation"
	tlsvalidationmock "github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation/mock"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretsstorevclient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

const (
	clusterName        = "AnyClusterName"
	anyAppName         = "any-app"
	anyComponentName   = "app"
	anyJobName         = "job"
	anyEnvironment     = "dev"
	anyEnvironmentName = "TEST_SECRET"
	subscriptionId     = "12347718-c8f8-4995-bfbb-02655ff1f89c"
)

func setupTest(t *testing.T, tlsValidator tlsvalidation.Validator) (*commontest.Utils, *controllertest.Utils, kubernetes.Interface, radixclient.Interface, prometheusclient.Interface, secretsstorevclient.Interface) {
	// Setup
	kubeclient := kubefake.NewSimpleClientset() //nolint:staticcheck
	radixclient := fake.NewSimpleClientset()    //nolint:staticcheck
	kedaClient := kedafake.NewSimpleClientset()
	prometheusclient := prometheusfake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	certClient := certclientfake.NewSimpleClientset()
	// commonTestUtils is used for creating CRDs
	commonTestUtils := commontest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient)
	err := commonTestUtils.CreateClusterPrerequisites(clusterName, subscriptionId)
	require.NoError(t, err)

	// secretControllerTestUtils is used for issuing HTTP request and processing responses
	mockValidator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	mockValidator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).AnyTimes().Return(controllertest.NewTestPrincipal(true), nil)
	secretControllerTestUtils := controllertest.NewTestUtils(kubeclient, radixclient, kedaClient, secretproviderclient, certClient, nil, mockValidator, NewSecretController(tlsValidator))

	return &commonTestUtils, &secretControllerTestUtils, kubeclient, radixclient, prometheusclient, secretproviderclient
}

func executeUpdateComponentSecretTest(t *testing.T, oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string) (*httptest.ResponseRecorder, error) {
	return executeUpdateSecretTest(t,
		oldSecretValue,
		updateSecret,
		updateComponent,
		updateSecretName,
		updateSecretValue,
		configureApplicationComponentSecret)
}

func executeUpdateJobSecretTest(t *testing.T, oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string) (*httptest.ResponseRecorder, error) {
	return executeUpdateSecretTest(t, oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue, configureApplicationJobSecret)
}

func configureApplicationComponentSecret(builder *operatorutils.ApplicationBuilder) {
	(*builder).WithComponents(
		operatorutils.AnApplicationComponent().
			WithName(anyComponentName).
			WithSecrets(anyEnvironmentName),
	)
}

func configureApplicationJobSecret(builder *operatorutils.ApplicationBuilder) {
	(*builder).WithJobComponents(
		operatorutils.AnApplicationJobComponent().
			WithName(anyJobName).
			WithSecrets(anyEnvironmentName),
	)
}

func executeUpdateSecretTest(t *testing.T, oldSecretValue, updateSecret, updateComponent, updateSecretName, updateSecretValue string, appConfigurator func(builder *operatorutils.ApplicationBuilder)) (*httptest.ResponseRecorder, error) {

	// Setup
	parameters := secretModels.SecretParameters{
		SecretValue: updateSecretValue,
	}

	commonTestUtils, controllerTestUtils, kubeclient, _, _, _ := setupTest(t, nil)
	appBuilder := operatorutils.
		ARadixApplication().
		WithAppName(anyAppName)
	appConfigurator(&appBuilder)

	_, err := commonTestUtils.ApplyApplication(appBuilder)
	if err != nil {
		return nil, err
	}
	ns := operatorutils.GetEnvironmentNamespace(anyAppName, anyEnvironment)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	_, err = kubeclient.CoreV1().Namespaces().Create(context.Background(), &namespace, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Component secret
	secretObject := corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyComponentName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	_, err = kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &secretObject, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Job secret
	secretObject = corev1.Secret{
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name: operatorutils.GetComponentSecretName(anyJobName),
		},
		Data: map[string][]byte{anyEnvironmentName: []byte(oldSecretValue)},
	}
	_, err = kubeclient.CoreV1().Secrets(ns).Create(context.Background(), &secretObject, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// Test
	responseChannel := controllerTestUtils.ExecuteRequestWithParameters("PUT", fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/secrets/%s", anyAppName, updateSecret, updateComponent, updateSecretName), parameters)
	response := <-responseChannel
	return response, nil
}

func TestUpdateSecret_OK(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingEnvironment_Missing2(t *testing.T) {
	nonExistingSecretName := "TEST"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, anyEnvironment, anyComponentName, nonExistingSecretName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, anyEnvironment, anyJobName, nonExistingSecretName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_EmptySecretValue_ValidationError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := ""

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "New secret value is empty", errorResponse.Message)
	assert.Equal(t, "Secret failed validation", errorResponse.Err.Error())
}

func TestUpdateSecret_NoUpdate_NoError(t *testing.T) {
	oldSecretValue := "oldvalue"
	updateSecretValue := "oldvalue"

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, anyEnvironment, anyComponentName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, anyEnvironment, anyJobName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestUpdateSecret_NonExistingComponent_Missing(t *testing.T) {
	nonExistingComponent := "frontend"
	nonExistingSecretObjName := operatorutils.GetComponentSecretName(nonExistingComponent)
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, anyEnvironment, nonExistingComponent, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", nonExistingSecretObjName), errorResponse.Err.Error())
}

func TestUpdateSecret_NonExistingEnvironment_Missing(t *testing.T) {
	nonExistingSecret := "prod"
	oldSecretValue := "oldvalue"
	updateSecretValue := "newvalue"

	response, err := executeUpdateComponentSecretTest(t, oldSecretValue, nonExistingSecret, anyComponentName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ := controllertest.GetErrorResponse(response)
	secretObjName := operatorutils.GetComponentSecretName(anyComponentName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())

	response, err = executeUpdateJobSecretTest(t, oldSecretValue, nonExistingSecret, anyJobName, anyEnvironmentName, updateSecretValue)
	require.NoError(t, err)
	errorResponse, _ = controllertest.GetErrorResponse(response)
	secretObjName = operatorutils.GetComponentSecretName(anyJobName)
	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, fmt.Sprintf("secrets \"%s\" not found", secretObjName), errorResponse.Err.Error())
}

type externalDNSSecretTestSuite struct {
	suite.Suite
	controllerTestUtils *controllertest.Utils
	commonTestUtils     *commontest.Utils
	tlsValidator        *tlsvalidationmock.MockValidator
	kubeClient          kubernetes.Interface
	radixClient         radixclient.Interface
}

func Test_ExternalDNSSecretTestSuite(t *testing.T) {
	suite.Run(t, new(externalDNSSecretTestSuite))
}

func (s *externalDNSSecretTestSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	s.tlsValidator = tlsvalidationmock.NewMockValidator(ctrl)
	s.commonTestUtils, s.controllerTestUtils, s.kubeClient, s.radixClient, _, _ = setupTest(s.T(), s.tlsValidator)
}

func (s *externalDNSSecretTestSuite) setupTestResources(appName, envName, componentName string, externalAliases []radixv1.RadixDeployExternalDNS, rdCondition radixv1.RadixDeployCondition) error {
	_, err := s.commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.NewDeploymentBuilder().
			WithRadixApplication(
				operatorutils.NewRadixApplicationBuilder().
					WithAppName(appName).
					WithRadixRegistration(
						operatorutils.NewRegistrationBuilder().
							WithName(appName),
					),
			).
			WithCondition(rdCondition).
			WithAppName(appName).
			WithEnvironment(envName).
			WithComponents(
				operatorutils.NewDeployComponentBuilder().
					WithName(componentName).
					WithExternalDNS(externalAliases...),
			),
	)

	return err
}

func (s *externalDNSSecretTestSuite) setupSecretForExternalDNS(namespace, fqdn string, cert []byte, privateKey []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: fqdn},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: cert, corev1.TLSPrivateKeyKey: privateKey},
	}
	_, err := s.kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	return err
}

func (s *externalDNSSecretTestSuite) executeRequest(appName, envName, componentName, fqdn string, body *secretModels.UpdateExternalDNSTLSRequest) *httptest.ResponseRecorder {
	endpoint := fmt.Sprintf("/api/v1/applications/%s/environments/%s/components/%s/externaldns/%s/tls", appName, envName, componentName, fqdn)
	responseCh := s.controllerTestUtils.ExecuteRequestWithParameters(http.MethodPut, endpoint, body)
	return <-responseCh
}

func (s *externalDNSSecretTestSuite) Test_UpdateSuccess() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	ns := operatorutils.GetEnvironmentNamespace(appName, envName)
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentActive))
	s.Require().NoError(s.setupSecretForExternalDNS(ns, fqdn, nil, nil))
	s.tlsValidator.EXPECT().ValidateX509Certificate([]byte(cert), []byte(privateKey), fqdn).Return(true, nil).Times(1)

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(200, response.Code)
	expectedSecretData := map[string][]byte{
		corev1.TLSCertKey:       []byte(cert),
		corev1.TLSPrivateKeyKey: []byte(privateKey),
	}
	secret, err := s.kubeClient.CoreV1().Secrets(ns).Get(context.Background(), fqdn, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedSecretData, secret.Data)

	metadata := kubequery.GetSecretMetadata(context.TODO(), secret)
	assert.WithinDuration(s.T(), time.Now(), pointers.Val(metadata.GetUpdated(corev1.TLSCertKey)), 1*time.Second)
	assert.WithinDuration(s.T(), time.Now(), pointers.Val(metadata.GetUpdated(corev1.TLSPrivateKeyKey)), 1*time.Second)
}

func (s *externalDNSSecretTestSuite) Test_SkipValidationDoesNotCallValidator() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	ns := operatorutils.GetEnvironmentNamespace(appName, envName)
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentActive))
	s.Require().NoError(s.setupSecretForExternalDNS(ns, fqdn, nil, nil))

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert, SkipValidation: true})
	s.Equal(200, response.Code)
	expectedSecretData := map[string][]byte{
		corev1.TLSCertKey:       []byte(cert),
		corev1.TLSPrivateKeyKey: []byte(privateKey),
	}
	secret, err := s.kubeClient.CoreV1().Secrets(ns).Get(context.Background(), fqdn, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(expectedSecretData, secret.Data)
}

func (s *externalDNSSecretTestSuite) Test_RadixDeploymentNotActive() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentInactive))

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(404, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal("Failed to get an active deployment from the deployment list", status.Message)
}

func (s *externalDNSSecretTestSuite) Test_NonExistingComponent() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	s.Require().NoError(s.setupTestResources(appName, envName, "othercomp", []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentActive))

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(404, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal(fmt.Sprintf("Component %q does not exist", componentName), status.Message)
}

func (s *externalDNSSecretTestSuite) Test_NonExistingExternalDNS() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: "other.example.com"}}, radixv1.DeploymentActive))

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(404, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal(fmt.Sprintf("External DNS %q not configured for component", fqdn), status.Message)
}

func (s *externalDNSSecretTestSuite) Test_ExternalDNSUsesAutomation() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn, UseCertificateAutomation: true}}, radixv1.DeploymentActive))

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(400, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal(fmt.Sprintf("External DNS %q is configured to use certificate automation", fqdn), status.Message)
}

func (s *externalDNSSecretTestSuite) Test_CertificateValidationError() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	validationMsg1, validationMsg2 := "validation error 1", "validation error 2"
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentActive))
	s.tlsValidator.EXPECT().ValidateX509Certificate([]byte(cert), []byte(privateKey), fqdn).Return(false, []string{validationMsg1, validationMsg2}).Times(1)

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(400, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal(fmt.Sprintf("%s, %s", validationMsg1, validationMsg2), status.Message)
	s.ErrorContains(status.Err, "TLS failed validation")
}

func (s *externalDNSSecretTestSuite) Test_NonExistingSecretFails() {
	appName, envName, componentName, fqdn := "app", "env", "comp", "my.example.com"
	privateKey, cert := "any private key", "any certificate"
	s.Require().NoError(s.setupTestResources(appName, envName, componentName, []radixv1.RadixDeployExternalDNS{{FQDN: fqdn}}, radixv1.DeploymentActive))
	s.tlsValidator.EXPECT().ValidateX509Certificate([]byte(cert), []byte(privateKey), fqdn).Return(true, nil).Times(1)

	response := s.executeRequest(appName, envName, componentName, fqdn, &secretModels.UpdateExternalDNSTLSRequest{PrivateKey: privateKey, Certificate: cert})
	s.Equal(500, response.Code)
	var status radixhttp.Error
	s.Require().NoError(controllertest.GetResponseBody(response, &status))
	s.Equal(fmt.Sprintf("Failed to update TLS private key and certificate for %q", fqdn), status.Message)
}
