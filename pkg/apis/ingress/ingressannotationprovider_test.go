package ingress

import (
	"errors"
	"testing"
	"time"

	maputils "github.com/equinor/radix-common/utils/maps"
	certificateconfig "github.com/equinor/radix-operator/pkg/apis/config/certificate"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func Test_NewForceSslRedirectAnnotationProvider(t *testing.T) {
	sut := NewForceSslRedirectAnnotationProvider()
	assert.IsType(t, &forceSslRedirectAnnotationProvider{}, sut)
}

func Test_NewIngressConfigurationAnnotationProvider(t *testing.T) {
	cfg := IngressConfiguration{AnnotationConfigurations: []AnnotationConfiguration{{Name: "test"}}}
	sut := NewIngressConfigurationAnnotationProvider(cfg)
	assert.IsType(t, &ingressConfigurationAnnotationProvider{}, sut)
	sutReal := sut.(*ingressConfigurationAnnotationProvider)
	assert.Equal(t, cfg, sutReal.config)
}

func Test_NewClientCertificateAnnotationProvider(t *testing.T) {
	expectedNamespace := "any-namespace"
	sut := NewClientCertificateAnnotationProvider(expectedNamespace)
	clientCertificateAnnotationProvider, converted := sut.(ClientCertificateAnnotationProvider)
	assert.True(t, converted, "Expected type ClientCertificateAnnotationProvider")
	assert.Equal(t, expectedNamespace, clientCertificateAnnotationProvider.GetNamespace())
}

func Test_NewOAuth2AnnotationProvider(t *testing.T) {
	cfg := defaults.MockOAuth2Config{}
	sut := NewOAuth2AnnotationProvider(&cfg)
	assert.IsType(t, &oauth2AnnotationProvider{}, sut)
	sutReal := sut.(*oauth2AnnotationProvider)
	assert.Equal(t, &cfg, sutReal.oauth2DefaultConfig)
}

func Test_ForceSslRedirectAnnotations(t *testing.T) {
	sslAnnotations := forceSslRedirectAnnotationProvider{}
	expected := map[string]string{"nginx.ingress.kubernetes.io/force-ssl-redirect": "true"}
	actual, err := sslAnnotations.GetAnnotations(&radixv1.RadixDeployComponent{}, "not-used-namespace-in-test")
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func Test_IngressConfigurationAnnotations(t *testing.T) {
	config := IngressConfiguration{
		AnnotationConfigurations: []AnnotationConfiguration{
			{Name: "ewma", Annotations: map[string]string{"ewma1": "x", "ewma2": "y"}},
			{Name: "socket", Annotations: map[string]string{"socket1": "x", "socket2": "y", "socket3": "z"}},
			{Name: "round-robin", Annotations: map[string]string{"round-robin1": "1"}},
		},
	}
	componentIngress := ingressConfigurationAnnotationProvider{config: config}

	annotations, err := componentIngress.GetAnnotations(&radixv1.RadixDeployComponent{IngressConfiguration: []string{"socket"}}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, 3, len(annotations))
	assert.Equal(t, config.AnnotationConfigurations[1].Annotations, annotations)

	annotations, err = componentIngress.GetAnnotations(&radixv1.RadixDeployComponent{IngressConfiguration: []string{"socket", "round-robin"}}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, 4, len(annotations))
	assert.Equal(t, maputils.MergeMaps(config.AnnotationConfigurations[1].Annotations, config.AnnotationConfigurations[2].Annotations), annotations)

	annotations, err = componentIngress.GetAnnotations(&radixv1.RadixDeployComponent{IngressConfiguration: []string{"non-existing"}}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(annotations))
}

func Test_ClientCertificateAnnotations(t *testing.T) {
	verification := radixv1.VerificationTypeOptional

	expect1 := make(map[string]string)
	expect1["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "true"
	expect1["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(radixv1.VerificationTypeOff)
	expect1["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	expect2 := make(map[string]string)
	expect2["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect2["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(radixv1.VerificationTypeOff)

	expect3 := make(map[string]string)
	expect3["nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream"] = "false"
	expect3["nginx.ingress.kubernetes.io/auth-tls-verify-client"] = string(verification)
	expect3["nginx.ingress.kubernetes.io/auth-tls-secret"] = utils.GetComponentClientCertificateSecretName("ns/name")

	config1 := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(true),
		},
	}

	config2 := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			PassCertificateToUpstream: utils.BoolPtr(false),
		},
	}

	config3 := &radixv1.Authentication{
		ClientCertificate: &radixv1.ClientCertificate{
			Verification: &verification,
		},
	}

	ingressAnnotations := NewClientCertificateAnnotationProvider("ns")
	result, err := ingressAnnotations.GetAnnotations(&radixv1.RadixDeployComponent{Name: "name", Authentication: config1}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, expect1, result)

	result, err = ingressAnnotations.GetAnnotations(&radixv1.RadixDeployComponent{Name: "name", Authentication: config2}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, expect2, result)

	result, err = ingressAnnotations.GetAnnotations(&radixv1.RadixDeployComponent{Name: "name", Authentication: config3}, "unused-namespace")
	assert.Nil(t, err)
	assert.Equal(t, expect3, result)

	result, err = ingressAnnotations.GetAnnotations(&radixv1.RadixDeployComponent{Name: "name"}, "unused-namespace")
	assert.Nil(t, err)
	assert.Empty(t, result, "Expected Annotations to be empty")
}

type OAuth2AnnotationsTestSuite struct {
	suite.Suite
	oauth2Config *defaults.MockOAuth2Config
	ctrl         *gomock.Controller
}

func TestOAuth2AnnotationsTestSuite(t *testing.T) {
	suite.Run(t, new(OAuth2AnnotationsTestSuite))
}

func (s *OAuth2AnnotationsTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.oauth2Config = defaults.NewMockOAuth2Config(s.ctrl)
}

func (s *OAuth2AnnotationsTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *OAuth2AnnotationsTestSuite) Test_NonPublicComponent() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(0)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{ClientID: "1234"}}}, "unused-namespace")
	s.Nil(err)
	s.Len(actual, 0)
}

func (s *OAuth2AnnotationsTestSuite) Test_PublicComponentNoOAuth() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(0)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{}}, "unused-namespace")
	s.Nil(err)
	s.Len(actual, 0)
}

func (s *OAuth2AnnotationsTestSuite) Test_ComponentOAuthPassedToOAuth2Config() {
	oauth := &radixv1.OAuth2{ClientID: "1234"}
	s.oauth2Config.EXPECT().MergeWith(oauth).Times(1).Return(&radixv1.OAuth2{}, nil)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	_, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: oauth}}, "unused-namespace")
	s.NoError(err)
}

func (s *OAuth2AnnotationsTestSuite) Test_AuthSigninAndUrlAnnotations() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{ProxyPrefix: "/anypath"}, nil)
	expected := map[string]string{
		"nginx.ingress.kubernetes.io/auth-signin": "https://$host/anypath/start?rd=$escaped_request_uri",
		"nginx.ingress.kubernetes.io/auth-url":    "http://oauth-test-aux-oauth.appname-namespace.svc.cluster.local:4180/anypath/auth",
	}
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{Name: "oauth-test", PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}}, "appname-namespace")
	s.Nil(err)
	s.Equal(expected, actual)
}

func (s *OAuth2AnnotationsTestSuite) Test_AuthResponseHeaderAnnotations_All() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(true), SetAuthorizationHeader: utils.BoolPtr(true)}, nil)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}}, "unused-namespace")
	s.Nil(err)
	s.Equal("X-Auth-Request-Access-Token,X-Auth-Request-User,X-Auth-Request-Groups,X-Auth-Request-Email,X-Auth-Request-Preferred-Username,Authorization", actual["nginx.ingress.kubernetes.io/auth-response-headers"])
}

func (s *OAuth2AnnotationsTestSuite) Test_AuthResponseHeaderAnnotations_XAuthHeadersOnly() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(true), SetAuthorizationHeader: utils.BoolPtr(false)}, nil)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}}, "unused-namespace")
	s.Nil(err)
	s.Equal("X-Auth-Request-Access-Token,X-Auth-Request-User,X-Auth-Request-Groups,X-Auth-Request-Email,X-Auth-Request-Preferred-Username", actual["nginx.ingress.kubernetes.io/auth-response-headers"])
}

func (s *OAuth2AnnotationsTestSuite) Test_AuthResponseHeaderAnnotations_AuthorizationHeaderOnly() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(false), SetAuthorizationHeader: utils.BoolPtr(true)}, nil)
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}}, "unused-namespace")
	s.Nil(err)
	s.Equal("Authorization", actual["nginx.ingress.kubernetes.io/auth-response-headers"])
}

func (s *OAuth2AnnotationsTestSuite) Test_OAuthConfig_ApplyTo_ReturnError() {
	s.oauth2Config.EXPECT().MergeWith(gomock.Any()).Times(1).Return(&radixv1.OAuth2{SetXAuthRequestHeaders: utils.BoolPtr(false), SetAuthorizationHeader: utils.BoolPtr(true)}, errors.New("any error"))
	sut := oauth2AnnotationProvider{oauth2DefaultConfig: s.oauth2Config}
	actual, err := sut.GetAnnotations(&radixv1.RadixDeployComponent{PublicPort: "http", Authentication: &radixv1.Authentication{OAuth2: &radixv1.OAuth2{}}}, "unused-namespace")
	s.Error(err)
	s.Nil(actual)
}

func Test_ExternalDNSAnnotationProvider(t *testing.T) {
	t.Run("expected annotations when useAnnotation is false", func(t *testing.T) {
		sut := NewExternalDNSAnnotationProvider(false, certificateconfig.AutomationConfig{})
		actualAnnotations, err := sut.GetAnnotations(nil, "")
		require.NoError(t, err)
		expectedAnnotations := map[string]string{kube.RadixExternalDNSUseCertificateAutomationAnnotation: "false"}
		assert.Equal(t, expectedAnnotations, actualAnnotations)
	})

	t.Run("expected annotations when useAnnotation is false", func(t *testing.T) {
		automationConfig := certificateconfig.AutomationConfig{
			ClusterIssuer: "anyclusterissuer",
			Duration:      4000*time.Hour - 1,
			RenewBefore:   1000 * time.Hour,
		}
		sut := NewExternalDNSAnnotationProvider(true, automationConfig)
		actualAnnotations, err := sut.GetAnnotations(nil, "")
		require.NoError(t, err)
		expectedAnnotations := map[string]string{
			kube.RadixExternalDNSUseCertificateAutomationAnnotation: "true",
			"cert-manager.io/cluster-issuer":                        automationConfig.ClusterIssuer,
			"cert-manager.io/duration":                              automationConfig.Duration.String(),
			"cert-manager.io/renew-before":                          automationConfig.RenewBefore.String(),
		}
		assert.Equal(t, expectedAnnotations, actualAnnotations)
	})

	t.Run("expected annotations when RenewBefore less than min value", func(t *testing.T) {
		automationConfig := certificateconfig.AutomationConfig{
			ClusterIssuer: "anyclusterissuer",
			Duration:      4000 * time.Hour,
			RenewBefore:   360*time.Hour - 1,
		}
		sut := NewExternalDNSAnnotationProvider(true, automationConfig)
		actualAnnotations, err := sut.GetAnnotations(nil, "")
		require.NoError(t, err)
		expectedAnnotations := map[string]string{
			kube.RadixExternalDNSUseCertificateAutomationAnnotation: "true",
			"cert-manager.io/cluster-issuer":                        automationConfig.ClusterIssuer,
			"cert-manager.io/duration":                              automationConfig.Duration.String(),
			"cert-manager.io/renew-before":                          (360 * time.Hour).String(),
		}
		assert.Equal(t, expectedAnnotations, actualAnnotations)
	})

	t.Run("expected annotations when Duration less than min value", func(t *testing.T) {
		automationConfig := certificateconfig.AutomationConfig{
			ClusterIssuer: "anyclusterissuer",
			Duration:      2160*time.Hour - 1,
			RenewBefore:   1000 * time.Hour,
		}
		sut := NewExternalDNSAnnotationProvider(true, automationConfig)
		actualAnnotations, err := sut.GetAnnotations(nil, "")
		require.NoError(t, err)
		expectedAnnotations := map[string]string{
			kube.RadixExternalDNSUseCertificateAutomationAnnotation: "true",
			"cert-manager.io/cluster-issuer":                        automationConfig.ClusterIssuer,
			"cert-manager.io/duration":                              (2160 * time.Hour).String(),
			"cert-manager.io/renew-before":                          automationConfig.RenewBefore.String(),
		}
		assert.Equal(t, expectedAnnotations, actualAnnotations)
	})

	t.Run("return error if cluster issuer is empty", func(t *testing.T) {
		automationConfig := certificateconfig.AutomationConfig{
			ClusterIssuer: "",
			Duration:      4000 * time.Hour,
			RenewBefore:   1000 * time.Hour,
		}
		sut := NewExternalDNSAnnotationProvider(true, automationConfig)
		_, err := sut.GetAnnotations(nil, "")
		assert.ErrorContains(t, err, "cluster issuer not set in certificate automation config")
	})
}
