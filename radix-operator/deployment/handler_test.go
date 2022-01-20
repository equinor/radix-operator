package deployment

import (
	"os"
	"testing"

	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	deployment "github.com/equinor/radix-operator/pkg/apis/deployment"
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
	kubeUtil             *kube.Kube
	eventRecorder        *record.FakeRecorder
}

func (s *handlerSuite) SetupTest() {
	s.kubeClient = fake.NewSimpleClientset()
	s.radixClient = fakeradix.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.secretProviderClient)
	s.promClient = prometheusfake.NewSimpleClientset()
	s.eventRecorder = &record.FakeRecorder{}
}

func (s *handlerSuite) Test_NewHandler_DefaultValues() {
	h := NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient)
	s.Equal(s.kubeClient, h.kubeclient)
	s.Equal(s.kubeUtil, h.kubeutil)
	s.Equal(s.radixClient, h.radixclient)
	s.Equal(s.promClient, h.prometheusperatorclient)
	s.False(h.forceRunAsNonRoot)
	s.NotNil(h.hasSynced)
}

func (s *handlerSuite) Test_NewHandler_ConfigOptionsCalled() {
	var called bool
	configFunc := func(h *Handler) { called = true }
	NewHandler(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, configFunc)
	s.True(called)
}

func (s *handlerSuite) Test_Sync() {
	nonExistingRdName, inactiveRdName, activeRdMissingRrName, activeRdName := "nonexisting-rd", "inactive-rd", "missingrr-rd", "active-rd"
	appName := "any-app"
	namespace := "any-ns"

	s.radixClient.Tracker().Add(
		&radixv1.RadixDeployment{
			ObjectMeta: v1.ObjectMeta{Name: inactiveRdName, Namespace: namespace},
			Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentInactive},
		},
	)
	s.radixClient.Tracker().Add(
		&radixv1.RadixDeployment{
			ObjectMeta: v1.ObjectMeta{Name: activeRdMissingRrName, Namespace: namespace},
			Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive},
		},
	)
	activeRd := &radixv1.RadixDeployment{
		ObjectMeta: v1.ObjectMeta{Name: activeRdName, Namespace: namespace},
		Spec:       radixv1.RadixDeploymentSpec{AppName: appName},
		Status:     radixv1.RadixDeployStatus{Condition: radixv1.DeploymentActive},
	}
	s.radixClient.Tracker().Add(activeRd)
	rr := &radixv1.RadixRegistration{
		ObjectMeta: v1.ObjectMeta{Name: appName},
	}
	s.radixClient.Tracker().Add(rr)

	s.Run("non-existing RD should not call factory method", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(0)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil}
		err := h.Sync(namespace, nonExistingRdName, s.eventRecorder)
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
		err := h.Sync(namespace, inactiveRdName, s.eventRecorder)
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
		err := h.Sync(namespace, activeRdMissingRrName, s.eventRecorder)
		s.NoError(err)
	})
	s.Run("active RD with existing RR calls factory with expected args", func() {
		var callbackExecuted bool
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		syncer := deployment.NewMockDeploymentSyncer(ctrl)
		syncer.EXPECT().OnSync().Times(1)
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		oauthConfig := defaults.OAuth2DefaultConfig{OAuth2: radixv1.OAuth2{ClientID: "1234"}}
		ingressConfig := deployment.IngressConfiguration{AnnotationConfigurations: []deployment.AnnotationConfiguration{{Name: "test"}}}
		expectedIngressAnnotations := []deployment.IngressAnnotationProvider{
			deployment.NewForceSslRedirectAnnotationProvider(),
			deployment.NewIngressConfigurationAnnotationProvider(ingressConfig),
			deployment.NewClientCertificateAnnotationProvider(activeRd.Namespace),
			deployment.NewOAuth2AnnotationProvider(&oauthConfig),
		}
		expectedAuxResources := []deployment.AuxiliaryResourceManager{
			deployment.NewOAuthProxyResourceManager(activeRd, rr, s.kubeUtil, &oauthConfig, []deployment.IngressAnnotationProvider{deployment.NewForceSslRedirectAnnotationProvider()}),
		}
		factory.
			EXPECT().
			CreateDeploymentSyncer(s.kubeClient, s.kubeUtil, s.radixClient, s.promClient, rr, activeRd, true, "1234", gomock.Eq(expectedIngressAnnotations), gomock.Eq(expectedAuxResources)).
			Return(syncer).
			Times(1)
		h := Handler{kubeclient: s.kubeClient, radixclient: s.radixClient, kubeutil: s.kubeUtil, prometheusperatorclient: s.promClient, deploymentSyncerFactory: factory, tenantId: "1234", forceRunAsNonRoot: true, oauth2DefaultConfig: oauthConfig, ingressConfiguration: ingressConfig, hasSynced: func(b bool) { callbackExecuted = b }}
		err := h.Sync(namespace, activeRdName, s.eventRecorder)
		s.NoError(err)
		s.True(callbackExecuted)
	})
	s.Run("active RD with exiting RR calls factory with non-root false", func() {
		ctrl := gomock.NewController(s.T())
		defer ctrl.Finish()
		syncer := deployment.NewMockDeploymentSyncer(ctrl)
		syncer.EXPECT().OnSync().Times(1)
		factory := deployment.NewMockDeploymentSyncerFactory(ctrl)
		factory.
			EXPECT().
			CreateDeploymentSyncer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false, gomock.Any(), gomock.Any(), gomock.Any()).
			Return(syncer).
			Times(1)
		h := Handler{radixclient: s.radixClient, kubeutil: s.kubeUtil, deploymentSyncerFactory: factory, forceRunAsNonRoot: false}
		err := h.Sync(namespace, activeRdName, s.eventRecorder)
		s.NoError(err)
	})
}

func Test_HandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerSuite))
}

func Test_WithForceRunAsNonRootFromEnvVar(t *testing.T) {
	os.Clearenv()
	os.Setenv("NONROOT1", "true")
	os.Setenv("NONROOT2", "false")

	h := &Handler{}
	WithForceRunAsNonRootFromEnvVar("NONROOT1")(h)
	assert.True(t, h.forceRunAsNonRoot)

	h = &Handler{}
	WithForceRunAsNonRootFromEnvVar("NONROOT2")(h)
	assert.False(t, h.forceRunAsNonRoot)

	h = &Handler{}
	WithForceRunAsNonRootFromEnvVar("NONROOT3")(h)
	assert.False(t, h.forceRunAsNonRoot)

	os.Clearenv()
}

func Test_WithTenantIdFromEnvVar(t *testing.T) {
	os.Clearenv()
	tenantId := "123456789-123456789"
	os.Setenv("RADIXOPERATOR_TENANT_ID", tenantId)

	h := &Handler{}
	WithTenantIdFromEnvVar("RADIXOPERATOR_TENANT_ID")(h)
	assert.Equal(t, tenantId, h.tenantId)

	os.Clearenv()
}

func Test_WithDeploymentSyncerFactory(t *testing.T) {
	factory := deployment.DeploymentSyncerFactoryFunc(deployment.NewDeployment)
	h := &Handler{}
	WithDeploymentSyncerFactory(factory)(h)
	assert.NotNil(t, h.deploymentSyncerFactory) // HACK Currently no way to compare function pointers
}

func Test_WithHasSyncedCallback(t *testing.T) {
	h := &Handler{}
	WithHasSyncedCallback(func(b bool) {})(h)
	assert.NotNil(t, h.hasSynced) // HACK Currently no way to compare function pointers
}
