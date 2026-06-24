package environments

import (
	"context"
	"fmt"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certclientfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-common/utils/pointers"
	deploymentModels "github.com/equinor/radix-operator/api-server/api/deployments/models"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	tlsvalidationmock "github.com/equinor/radix-operator/api-server/api/utils/tlsvalidation/mock"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	commontest "github.com/equinor/radix-operator/pkg/apis/test"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ExternalDnsAutomationTestSuite(t *testing.T) {
	suite.Run(t, new(externalDnsAutomationTestSuite))
}

type externalDnsAutomationTestSuite struct {
	suite.Suite
	commonTestUtils      *commontest.Utils
	environmentTestUtils *controllertest.Utils
	certClient           *certclientfake.Clientset
	appName              string
	environmentName      string
	fqdn                 string
}

func (s *externalDnsAutomationTestSuite) SetupTest() {
	ctrl := gomock.NewController(s.T())
	tlsValidator := tlsvalidationmock.NewMockValidator(ctrl)
	tlsValidator.EXPECT().ValidateX509Certificate(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	s.commonTestUtils, s.environmentTestUtils, _, _, _, _, _, _, s.certClient = setupTest(s.T(), []EnvironmentHandlerOptions{WithTLSValidator(tlsValidator)})

	s.appName, s.environmentName, s.fqdn = "any-app", "dev", "any.alias.com"
	componentName := "backend"

	_, err := s.commonTestUtils.ApplyDeployment(
		context.Background(),
		operatorutils.
			ARadixDeployment().
			WithAppName(s.appName).
			WithEnvironment(s.environmentName).
			WithComponents(operatorutils.NewDeployComponentBuilder().WithName(componentName).WithExternalDNS(radixv1.RadixDeployExternalDNS{FQDN: s.fqdn, UseCertificateAutomation: true})).
			WithJobComponents().
			WithImageTag("master"))
	require.NoError(s.T(), err)

	_, err = s.commonTestUtils.ApplyApplication(operatorutils.
		ARadixApplication().
		WithAppName(s.appName).
		WithEnvironment(s.environmentName, "master").
		WithComponents(operatorutils.
			AnApplicationComponent().
			WithName(componentName)))
	require.NoError(s.T(), err)
}

func (s *externalDnsAutomationTestSuite) environmentNamespace() string {
	return operatorutils.GetEnvironmentNamespace(s.appName, s.environmentName)
}

func (s *externalDnsAutomationTestSuite) executeRequest(appName, envName string) (environment *environmentModels.Environment, statusCode int, err error) {
	responseChannel := s.environmentTestUtils.ExecuteRequest("GET", fmt.Sprintf("/api/v1/applications/%s/environments/%s", appName, envName))
	response := <-responseChannel
	var env environmentModels.Environment
	err = controllertest.GetResponseBody(response, &env)
	if err == nil {
		environment = &env
	}
	statusCode = response.Code
	return
}

func (s *externalDnsAutomationTestSuite) Test_CertReady() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionTrue}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationSuccess,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertReady_IncorrectAppNameLabel() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: "otherapp", kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionTrue}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertReady_IncorrectFQDNLabel() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: "other-fqdn"}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionTrue}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertReady_WrongNamespace() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionTrue}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates("other-ns").Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertReady_StatusFalse() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionFalse}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertReady_StatusUnknown() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionReady, Status: v1.ConditionUnknown}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_Cert_MissingReadyCondition() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Conditions: []cmv1.CertificateCondition{{Type: cmv1.CertificateConditionIssuing, Status: v1.ConditionTrue}}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertMissing() {
	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReady_SameRevision() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReadyNoRevision_RequestReady_Revision1() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "1"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_MultipleRequests_SameOrHigherRevision() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq1 := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "certreq1",
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "6"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: "other msg"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq1, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq2 := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "certreq2",
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "7"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq2, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq3 := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "certreq3",
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: "other msg"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq3, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReady_LowerRevision() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "4"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: "any message"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReady_InvalidRevision() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "non-numeric"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: "any message"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReady_NotOwnedByCert() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: "otheruid", Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Message: "any message"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestDenied() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionDenied, Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationFailed,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestInvalidTrue() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionInvalidRequest, Status: v1.ConditionTrue, Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationFailed,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestInvalidFalse() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionInvalidRequest, Status: v1.ConditionFalse, Message: "any message"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestInvalidUnknown() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionInvalidRequest, Status: v1.ConditionUnknown, Message: "any message"}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: "",
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReadyFalse_ReasonFailed() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Status: v1.ConditionFalse, Reason: "Failed", Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationFailed,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReadyUnknown_ReasonFailed() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Status: v1.ConditionUnknown, Reason: "Failed", Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationFailed,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}

func (s *externalDnsAutomationTestSuite) Test_CertNotReady_RequestReadyTrue_ReasonFailed() {
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{UID: "uid1", Labels: map[string]string{kube.RadixAppLabel: s.appName, kube.RadixExternalAliasFQDNLabel: s.fqdn}},
		Status:     cmv1.CertificateStatus{Revision: pointers.Ptr(5)},
	}
	_, err := s.certClient.CertmanagerV1().Certificates(s.environmentNamespace()).Create(context.Background(), cert, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedRequestMsg := "any message"
	certReq := &cmv1.CertificateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:     map[string]string{"cert-manager.io/certificate-revision": "5"},
			OwnerReferences: []metav1.OwnerReference{{UID: cert.UID, Controller: pointers.Ptr(true)}},
		},
		Status: cmv1.CertificateRequestStatus{Conditions: []cmv1.CertificateRequestCondition{{Type: cmv1.CertificateRequestConditionReady, Status: v1.ConditionTrue, Reason: "Failed", Message: expectedRequestMsg}}},
	}
	_, err = s.certClient.CertmanagerV1().CertificateRequests(s.environmentNamespace()).Create(context.Background(), certReq, metav1.CreateOptions{})
	s.Require().NoError(err)

	environment, statusCode, err := s.executeRequest(s.appName, s.environmentName)
	s.Equal(statusCode, 200)
	s.NoError(err)

	expectedExternalDNS := deploymentModels.TLSAutomation{
		Status:  deploymentModels.TLSAutomationPending,
		Message: expectedRequestMsg,
	}
	s.Equal(expectedExternalDNS, *environment.ActiveDeployment.Components[0].ExternalDNS[0].TLS.Automation)
}
