package deployment

import (
	"context"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	certfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	fakeradix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

type handlerSuite struct {
	suite.Suite
	kubeClient           *fake.Clientset
	radixClient          *fakeradix.Clientset
	secretProviderClient *secretproviderfake.Clientset
	promClient           *prometheusfake.Clientset
	certClient           *certfake.Clientset
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
	kedaClient           *kedafake.Clientset
	config               *config.Config
}

func Test_HandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerSuite))
}

func (s *handlerSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.certClient = certfake.NewSimpleClientset()
	s.config = &config.Config{LogLevel: "some_non_default_value"} // Add a non-default value since gomock uses DeepEqual for equality compare instead of pointer equality
	s.eventRecorder = &record.FakeRecorder{}
}

func (s *handlerSuite) Test_NewHandler_DefaultValues() {
	h := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.kedaClient, s.promClient, s.certClient, s.config)
	s.Same(s.kubeClient, h.kubeclient)
	s.Same(s.kubeUtil, h.kubeutil)
	s.Same(s.radixClient, h.radixclient)
	s.Same(s.promClient, h.prometheusperatorclient)
	s.Same(s.certClient, h.certClient)
	s.Same(s.config, h.config)
	s.NotNil(h.hasSynced)
}

func (s *handlerSuite) Test_NewHandler_ConfigOptionsCalled() {
	var called bool
	configFunc := func(h *Handler) { called = true }
	NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.kedaClient, s.promClient, s.certClient, &config.Config{}, configFunc)
	s.True(called)
}

func (s *handlerSuite) Test_Sync() {
	nonExistingRdName, inactiveRdName, activeRdMissingRrName, activeRdName := "nonexisting-rd", "inactive-rd", "missingrr-rd", "active-rd"
	appName := "any-app"
	namespace := "any-ns"

	err := s.radixClient.Tracker().Add(
		&radixv1.RadixDeployment{
			ObjectMeta: v1.ObjectMeta{Name: inactiveRdName, Namespace: namespace},
			Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentInactive},
		},
	)
	s.Require().NoError(err)

	err = s.radixClient.Tracker().Add(
		&radixv1.RadixDeployment{
			ObjectMeta: v1.ObjectMeta{Name: activeRdMissingRrName, Namespace: namespace},
			Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive},
		},
	)
	s.Require().NoError(err)

	activeRd := &radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Name: activeRdName, Namespace: namespace},
		Spec:       radixv1.RadixDeploymentSpec{AppName: appName},
		Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive},
	}
	err = s.radixClient.Tracker().Add(activeRd)
	s.Require().NoError(err)

	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Name: appName},
	}
	err = s.radixClient.Tracker().Add(rr)
	s.Require().NoError(err)

	s.Run("non-existing RD should not call factory method", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(0)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil}
		err := h.Sync(context.Background(), namespace, nonExistingRdName, s.eventRecorder)
		s.NoError(err)
	})
	s.Run("inactive RD should not call factory method", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(0)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil}
		err := h.Sync(context.Background(), namespace, inactiveRdName, s.eventRecorder)
		s.NoError(err)
	})
	s.Run("active RD with missing RR should not call factory method", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(0)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil}
		err := h.Sync(context.Background(), namespace, activeRdMissingRrName, s.eventRecorder)
		s.NoError(err)
	})
	s.Run("active RD with existing RR calls factory with expected args", func() {
		var callbackExecuted bool
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		syncer := deployment.NewMockDeploymentSyncer(ctrl)
		syncer.EXPECT().OnSync(gomock.Any()).Times(1)
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		oauthConfig := defaults.NewOAuth2Config()
		ingressConfig := ingress.IngressConfiguration{AnnotationConfigurations: []ingress.AnnotationConfiguration{{Name: "test"}}}
		expectedIngressAnnotations := []ingress.AnnotationProvider{
			ingress.NewForceSslRedirectAnnotationProvider(),
			ingress.NewIngressConfigurationAnnotationProvider(ingressConfig),
			ingress.NewClientCertificateAnnotationProvider(activeRd.Namespace),
			ingress.NewOAuth2AnnotationProvider(oauthConfig),
		}
		expectedAuxResources := []deployment.AuxiliaryResourceManager{
			deployment.NewOAuthProxyResourceManager(activeRd, rr, s.kubeUtil, oauthConfig, ingress.GetAuxOAuthProxyAnnotationProviders(), "oauth:123"),
		}
		factory.
			EXPECT().
			CreateDeploymentSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, s.certClient, rr, activeRd, gomock.Eq(expectedIngressAnnotations), gomock.Eq(expectedAuxResources), s.config).
			Return(syncer).
			Times(1)
		h := Handler{kubeclient: s.kubeClient, radixclient: s.radixClient, kubeutil: s.kubeUtil, kedaClient: s.kedaClient, prometheusperatorclient: s.promClient, certClient: s.certClient, deploymentSyncerFactory: factory, oauth2ProxyDockerImage: "oauth:123", oauth2DefaultConfig: oauthConfig, ingressConfiguration: ingressConfig, hasSynced: func(b bool) { callbackExecuted = b }, config: s.config}
		err := h.Sync(context.Background(), namespace, activeRdName, s.eventRecorder)
		s.NoError(err)
		s.True(callbackExecuted)
	})
	s.Run("active RD with exiting RR calls factory with non-root false", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		syncer := deployment.NewMockDeploymentSyncer(ctrl)
		syncer.EXPECT().OnSync(gomock.Any()).Times(1)
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(syncer).
			Times(1)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil, kedaClient: s.kedaClient, deploymentSyncerFactory: factory}
		err := h.Sync(context.Background(), namespace, activeRdName, s.eventRecorder)
		s.NoError(err)
	})
}

func Test_WithDeploymentSyncerFactory(t *testing.T) {
	factory := deployment.DeploymentSyncerFactoryFunc(deployment.NewDeploymentSyncer)
	h := &Handler{}
	WithDeploymentSyncerFactory(factory)(h)
	assert.NotNil(t, h.deploymentSyncerFactory) // HACK Currently no way to compare function pointers
}

func Test_WithHasSyncedCallback(t *testing.T) {
	h := &Handler{}
	WithHasSyncedCallback(func(b bool) {})(h)
	assert.NotNil(t, h.hasSynced) // HACK Currently no way to compare function pointers
}
