package deployment

import (
	"context"
	"reflect"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults/k8s"
	"github.com/equinor/radix-operator/pkg/apis/ingress"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/golang/mock/gomock"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type OAuthRedisResourceManagerTestSuite struct {
	suite.Suite
	kubeClient           kubernetes.Interface
	radixClient          radixclient.Interface
	kedaClient           kedav2.Interface
	secretProviderClient secretProviderClient.Interface
	kubeUtil             *kube.Kube
	ctrl                 *gomock.Controller
}

func TestOAuthRedisResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthRedisResourceManagerTestSuite))
}

func (s *OAuthRedisResourceManagerTestSuite) SetupSuite() {
	s.T().Setenv(defaults.OperatorDefaultUserGroupEnvironmentVariable, "1234-5678-91011")
	s.T().Setenv(defaults.OperatorAppAliasBaseURLEnvironmentVariable, "app.dev.radix.equinor.com")
	s.T().Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "300M")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	s.T().Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	s.T().Setenv(defaults.OperatorReadinessProbeInitialDelaySeconds, "5")
	s.T().Setenv(defaults.OperatorReadinessProbePeriodSeconds, "10")
	s.T().Setenv(defaults.OperatorRadixJobSchedulerEnvironmentVariable, "radix-job-scheduler:main-latest")
	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, "development")
	s.T().Setenv(defaults.RadixOAuthProxyDefaultOIDCIssuerURLEnvironmentVariable, "oidc_issuer_url")
}

func (s *OAuthRedisResourceManagerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *OAuthRedisResourceManagerTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset()
	s.kedaClient = kedafake.NewSimpleClientset()
	s.secretProviderClient = secretproviderfake.NewSimpleClientset()
	s.kubeUtil, _ = kube.New(s.kubeClient, s.radixClient, s.kedaClient, s.secretProviderClient)
	s.ctrl = gomock.NewController(s.T())
}

func (s *OAuthRedisResourceManagerTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *OAuthRedisResourceManagerTestSuite) TestNewOAuthRedisResourceManager() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	rd := utils.NewDeploymentBuilder().BuildRD()
	rr := utils.NewRegistrationBuilder().BuildRR()

	oauthManager := NewOAuthRedisResourceManager(rd, rr, s.kubeUtil, "redis:123")
	sut, ok := oauthManager.(*oauthRedisResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
	s.Equal("redis:123", sut.oauthRedisDockerImage)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_ComponentRestartEnvVar() {
	auth := &v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234", SessionStoreType: v1.SessionStoreSystemManaged}}
	appName := "anyapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}
	type testSpec struct {
		name                string
		rd                  *v1.RadixDeployment
		expectRestartEnvVar bool
	}
	tests := []testSpec{
		{
			name: "component default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithEnvironmentVariable(defaults.RadixRestartEnvironmentVariable, "anyval")).
				BuildRD(),
			expectRestartEnvVar: true,
		},
		{
			name: "component replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectRestartEnvVar: false,
		},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			envVarExist := slice.Any(deploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool { return ev.Name == defaults.RadixRestartEnvironmentVariable })
			s.Equal(test.expectRestartEnvVar, envVarExist)
		})
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_NotPublicOrNoOAuth() {
	appName := "anyapp"
	type scenarioDef struct{ rd *v1.RadixDeployment }
	scenarios := []scenarioDef{
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithPublicPort("http")).BuildRD()},
		{rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").WithComponent(utils.NewDeployComponentBuilder().WithName("nooauth").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{
			ClientID:         "1234",
			SessionStoreType: v1.SessionStoreSystemManaged,
		}})).BuildRD()},
	}
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()

	for _, scenario := range scenarios {
		sut := &oauthRedisResourceManager{scenario.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
		err := sut.Sync(context.Background())
		s.Nil(err)
		deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(deploys.Items, 0)
		services, _ := s.kubeClient.CoreV1().Services(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
		s.Len(services.Items, 0)
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OauthDeploymentReplicas() {
	auth := &v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234", SessionStoreType: v1.SessionStoreSystemManaged}}
	appName := "anyapp"
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	baseComp := func() utils.DeployComponentBuilder {
		return utils.NewDeployComponentBuilder().WithName("comp").WithPublicPort("http").WithAuthentication(auth)
	}
	type testSpec struct {
		name             string
		rd               *v1.RadixDeployment
		expectedReplicas int32
	}
	tests := []testSpec{
		{
			name: "component default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp()).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		{
			name: "component replicas set override to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		{
			name: "component with hpa and default config",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component with hpa and replicas set to 1",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component with hpa and replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(2)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{

			name: "component with hpa and replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(pointers.Ptr(1)).WithReplicasOverride(pointers.Ptr(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 0,
		},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			s.setupTest()
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := sut.kubeutil.KubeClient().AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			s.Equal(test.expectedReplicas, *deploys.Items[0].Spec.Replicas)
		})
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisDeploymentCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)
	inputOAuth := &v1.OAuth2{ClientID: "1234", SessionStoreType: v1.SessionStoreSystemManaged}

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: inputOAuth}).WithRuntime(&v1.Runtime{Architecture: "customarch"})).
		BuildRD()

	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Require().NoError(err, "failed to sync oauth redis manager")

	actualDeploys, _ := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(actualDeploys.Items, 1)

	actualDeploy, ok := slice.FindFirst(actualDeploys.Items, func(deploy appsv1.Deployment) bool {
		return deploy.Name == utils.GetAuxiliaryComponentDeploymentName(componentName, v1.OAuthRedisAuxiliaryComponentSuffix)
	})
	s.Require().True(ok, "oauth2 redis deployment not found")
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualDeploy.OwnerReferences)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	s.Equal(expectedLabels, actualDeploy.Labels)
	s.Len(actualDeploy.Spec.Template.Spec.Containers, 1)
	s.Equal(expectedLabels, actualDeploy.Spec.Template.Labels)
	redisDataVolume, redisDataVolumeExists := slice.FindFirst(actualDeploy.Spec.Template.Spec.Volumes, func(volume corev1.Volume) bool { return volume.Name == "redis-data" })
	s.True(redisDataVolumeExists, "Missing volume redis-data")
	s.NotNil(redisDataVolume.EmptyDir, "Missing EmptyDir in the volume redis-data")

	defaultContainer := actualDeploy.Spec.Template.Spec.Containers[0]
	s.Equal(sut.oauthRedisDockerImage, defaultContainer.Image)

	s.Len(defaultContainer.Ports, 1)
	s.Equal(v1.OAuthRedisPortNumber, defaultContainer.Ports[0].ContainerPort)
	s.Equal(v1.OAuthRedisPortName, defaultContainer.Ports[0].Name)
	s.NotNil(defaultContainer.ReadinessProbe)
	s.Equal(v1.OAuthRedisPortNumber, defaultContainer.ReadinessProbe.TCPSocket.Port.IntVal)

	expectedArgs := []string{
		"redis-server",
		"--save", "3600 1 300 100 60 10000",
		"--dir", "/data",
		"--appendonly", "yes",
		"--protected-mode", "yes",
		"--requirepass", "$(REDIS_PASSWORD)",
	}
	s.Equal(expectedArgs, defaultContainer.Args, "Unexpected args in oauth redis container")

	redisDataVolumeMount, redisDataVolumeMountExists := slice.FindFirst(defaultContainer.VolumeMounts, func(volumeMount corev1.VolumeMount) bool { return volumeMount.Name == "redis-data" })
	s.True(redisDataVolumeMountExists, "Missing volume redis-data")
	s.Equal("/data", redisDataVolumeMount.MountPath, "Missing EmptyDir in the volume-mount redis-data")

	expectedAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
			{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
			{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorArchitecture}},
		}}}}},
	}
	s.Equal(expectedAffinity, actualDeploy.Spec.Template.Spec.Affinity, "oauth2 aux deployment must not use component's runtime config")

	s.Len(defaultContainer.Env, 1)
	secretName := utils.GetAuxiliaryComponentSecretName(componentName, v1.OAuthProxyAuxiliaryComponentSuffix)
	s.Equal(corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: defaults.OAuthRedisPasswordKeyName, LocalObjectReference: corev1.LocalObjectReference{Name: secretName}}}, s.getEnvVarValueFromByName(redisPasswordEnvironmentVariable, defaultContainer.Env))
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisServiceCreated() {
	appName, envName, componentName := "anyapp", "qa", "server"
	envNs := utils.GetEnvironmentNamespace(appName, envName)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(componentName).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreSystemManaged}})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.Nil(err)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: componentName, kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	expectedServiceName := utils.GetAuxOAuthRedisServiceName(componentName)
	actualServices, _ := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.Len(actualServices.Items, 1)
	s.Equal(expectedServiceName, actualServices.Items[0].Name)
	s.Equal(expectedLabels, actualServices.Items[0].Labels)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, actualServices.Items[0].OwnerReferences)
	s.Equal(corev1.ServiceTypeClusterIP, actualServices.Items[0].Spec.Type)
	s.Len(actualServices.Items[0].Spec.Ports, 1)
	s.Equal(corev1.ServicePort{Port: v1.OAuthRedisPortNumber, TargetPort: intstr.FromInt32(v1.OAuthRedisPortNumber), Protocol: corev1.ProtocolTCP}, actualServices.Items[0].Spec.Ports[0])
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_OAuthRedisUninstall() {
	appName, envName, component1Name, component2Name := "anyapp", "qa", "server", "web"
	envNs := utils.GetEnvironmentNamespace(appName, envName)

	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreSystemManaged}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreSystemManaged}})).
		BuildRD()
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err := sut.Sync(context.Background())
	s.NoError(err, "failed to sync oauth redis manager")

	// Bootstrap oauth redis resources for two components
	actualDeploys, err := s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list deployments")
	s.Len(actualDeploys.Items, 2)
	actualServices, err := s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list services")
	s.Len(actualServices.Items, 2)

	// Set OAuth config to nil for second component
	rd = utils.NewDeploymentBuilder().
		WithAppName(appName).
		WithEnvironment(envName).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component1Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{SessionStoreType: v1.SessionStoreSystemManaged}})).
		WithComponent(utils.NewDeployComponentBuilder().WithName(component2Name).WithPublicPort("http").WithAuthentication(&v1.Authentication{})).
		BuildRD()
	sut = &oauthRedisResourceManager{rd, rr, s.kubeUtil, "redis:123", zerolog.Nop()}
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, err = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list deployments after OAuth config change")
	s.Len(actualDeploys.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(component1Name, v1.OAuthRedisAuxiliaryComponentSuffix), actualDeploys.Items[0].Name)
	actualServices, err = s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list services after OAuth config change")
	s.Len(actualServices.Items, 1)
	s.Equal(utils.GetAuxOAuthRedisServiceName(component1Name), actualServices.Items[0].Name)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_GetOwnerReferenceOfIngress() {
	actualOwnerReferences := ingress.GetOwnerReferenceOfIngress(&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "anyingress", UID: "anyuid"}})
	s.ElementsMatch([]metav1.OwnerReference{{APIVersion: networkingv1.SchemeGroupVersion.Identifier(), Kind: k8s.KindIngress, Name: "anyingress", UID: "anyuid", Controller: pointers.Ptr(true)}}, actualOwnerReferences)
}

func (s *OAuthRedisResourceManagerTestSuite) Test_GarbageCollect() {

	rd := utils.NewDeploymentBuilder().
		WithAppName("myapp").
		WithEnvironment("dev").
		WithComponent(utils.NewDeployComponentBuilder().WithName("c1")).
		WithComponent(utils.NewDeployComponentBuilder().WithName("c2")).
		BuildRD()

	s.addDeployment("d1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addDeployment("d6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addDeployment("d7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	s.addService("svc1", "myapp-dev", "myapp", "c1", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc2", "myapp-dev", "myapp", "c2", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc3", "myapp-dev", "myapp", "c3", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc4", "myapp-dev", "myapp", "c4", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc5", "myapp-dev", "myapp", "c5", "anyauxtype")
	s.addService("svc6", "myapp-dev", "myapp2", "c6", v1.OAuthRedisAuxiliaryComponentType)
	s.addService("svc7", "myapp-qa", "myapp", "c7", v1.OAuthRedisAuxiliaryComponentType)

	sut := oauthRedisResourceManager{rd: rd, kubeutil: s.kubeUtil}
	err := sut.GarbageCollect(context.Background())
	s.Nil(err)

	actualDeployments, err := s.kubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err, "failed to list deployments")
	s.Len(actualDeployments.Items, 5)
	s.ElementsMatch([]string{"d1", "d2", "d5", "d6", "d7"}, s.getObjectNames(actualDeployments.Items))

	actualServices, err := s.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	s.Require().NoError(err, "failed to list services")
	s.Len(actualServices.Items, 5)
	s.ElementsMatch([]string{"svc1", "svc2", "svc5", "svc6", "svc7"}, s.getObjectNames(actualServices.Items))
}

func (*OAuthRedisResourceManagerTestSuite) getEnvVarValueFromByName(name string, envvars []corev1.EnvVar) corev1.EnvVarSource {
	for _, envvar := range envvars {
		if envvar.Name == name && envvar.ValueFrom != nil {
			return *envvar.ValueFrom
		}
	}
	return corev1.EnvVarSource{}
}

func (s *OAuthRedisResourceManagerTestSuite) getObjectNames(items interface{}) []string {
	tItems := reflect.TypeOf(items)
	if tItems.Kind() != reflect.Slice {
		return nil
	}

	var names []string
	vItems := reflect.ValueOf(items)
	for i := 0; i < vItems.Len(); i++ {
		v := vItems.Index(i).Addr().Interface()
		if o, ok := v.(metav1.Object); ok {
			names = append(names, o.GetName())
		}

	}

	return names
}

func (*OAuthRedisResourceManagerTestSuite) getAppNameSelector(appName string) string {
	r, _ := labels.NewRequirement(kube.RadixAppLabel, selection.Equals, []string{appName})
	return r.String()
}

func (s *OAuthRedisResourceManagerTestSuite) addDeployment(name, namespace, appName, auxComponentName, auxComponentType string) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthRedisResourceManagerTestSuite) addService(name, namespace, appName, auxComponentName, auxComponentType string) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    s.buildResourceLabels(appName, auxComponentName, auxComponentType),
		},
	}
	_, err := s.kubeClient.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *OAuthRedisResourceManagerTestSuite) buildResourceLabels(appName, auxComponentName, auxComponentType string) labels.Set {
	return map[string]string{
		kube.RadixAppLabel:                    appName,
		kube.RadixAuxiliaryComponentLabel:     auxComponentName,
		kube.RadixAuxiliaryComponentTypeLabel: auxComponentType,
	}
}
