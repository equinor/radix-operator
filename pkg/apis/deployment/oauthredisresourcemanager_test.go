package deployment

import (
	"context"
	"reflect"
	"testing"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	cfg                  config.Config
}

func TestOAuthRedisResourceManagerTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthRedisResourceManagerTestSuite))
}

func (s *OAuthRedisResourceManagerTestSuite) SetupSuite() {
	s.cfg = config.Config{
		Common: config.CommonConfig{
			AppAliasBaseURL:            "app.dev.radix.equinor.com",
			ExternalRegistryAuthSecret: "someSecret",
		},
		Runtime: config.RuntimeConfig{
			Oauth2SessionStoreTemplate: config.RuntimeBaseOverlayPodConfig{
				Base: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            SessionStoreContainerName,
								Image:           "someredisimage:latest",
								ImagePullPolicy: corev1.PullAlways,
							},
						},
					},
				},
				Overlay: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  SessionStoreContainerName,
							Image: "someredisimage:v1.2.3",
						}},
					},
				},
			},
		},
	}
}

func (s *OAuthRedisResourceManagerTestSuite) SetupTest() {
	s.setupTest()
}

func (s *OAuthRedisResourceManagerTestSuite) setupTest() {
	s.kubeClient = kubefake.NewSimpleClientset()
	s.radixClient = radixfake.NewSimpleClientset() // nolint:staticcheck // SA1019: Ignore linting deprecated fields
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

	oauthManager := NewOAuthRedisResourceManager(rd, rr, s.kubeUtil, s.cfg)
	sut, ok := oauthManager.(*oauthRedisResourceManager)
	s.True(ok)
	s.Equal(rd, sut.rd)
	s.Equal(rr, sut.rr)
	s.Equal(s.kubeUtil, sut.kubeutil)
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
				WithComponent(baseComp().WithReplicas(new(1))).
				BuildRD(),
			expectRestartEnvVar: false,
		},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			envVarExist := slice.Any(deploys.Items[0].Spec.Template.Spec.Containers[0].Env, func(ev corev1.EnvVar) bool { return ev.Name == defaults.RadixRestartEnvironmentVariable })
			s.Equal(test.expectRestartEnvVar, envVarExist)
		})
	}
}

func (s *OAuthRedisResourceManagerTestSuite) Test_Sync_PodTemplateFromConfigIsMerged() {
	appName := "anyapp"
	compBuilder := utils.NewDeployComponentBuilder().
		WithName("comp").
		WithPublicPort("http").
		WithAuthentication(&v1.Authentication{OAuth2: &v1.OAuth2{ClientID: "1234", SessionStoreType: v1.SessionStoreSystemManaged}})
	rr := utils.NewRegistrationBuilder().WithName(appName).BuildRR()
	rd := utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
		WithComponent(compBuilder).
		BuildRD()

	secretName := utils.GetAuxiliaryComponentSecretName("comp", v1.OAuthProxyAuxiliaryComponentSuffix)
	expectedPodTemplate := corev1.PodTemplateSpec{
		Labels: radixlabels.ForAuxOAuthRedisComponent(appName, new(compBuilder.BuildComponent())),
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: s.cfg.Common.ExternalRegistryAuthSecret},
			},
			Containers: []corev1.Container{
				{
					Name:            SessionStoreContainerName,
					Image:           "someredisimage:v1.2.3",
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{{
						Name: redisPasswordEnvironmentVariable,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Name: secretName,
								Key:  defaults.OAuthRedisPasswordKeyName,
							},
						},
					}},
				},
			},
		},
	}

	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
	s.Require().NoError(sut.Sync(context.Background()))

	deploys, err := s.kubeClient.AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
	s.Require().NoError(err)
	s.Require().Len(deploys.Items, 1)

	expectedLabels := map[string]string{kube.RadixAppLabel: appName, kube.RadixAuxiliaryComponentLabel: "comp", kube.RadixAuxiliaryComponentTypeLabel: v1.OAuthRedisAuxiliaryComponentType}
	s.Equal(expectedLabels, deploys.Items[0].Labels)
	s.ElementsMatch([]metav1.OwnerReference{getOwnerReferenceOfDeployment(rd)}, deploys.Items[0].OwnerReferences)

	podSpec := deploys.Items[0].Spec.Template
	s.Equal(expectedPodTemplate, podSpec)
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
		sut := &oauthRedisResourceManager{scenario.rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
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
				WithComponent(baseComp().WithReplicas(new(1))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(new(2))).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(new(0))).
				BuildRD(),
			expectedReplicas: 0,
		},
		{
			name: "component replicas set override to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(new(1)).WithReplicasOverride(new(0))).
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
				WithComponent(baseComp().WithReplicas(new(1)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{
			name: "component with hpa and replicas set to 2",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(new(2)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 1,
		},
		{

			name: "component with hpa and replicas set to 0",
			rd: utils.NewDeploymentBuilder().WithAppName(appName).WithEnvironment("qa").
				WithComponent(baseComp().WithReplicas(new(1)).WithReplicasOverride(new(0)).WithHorizontalScaling(utils.NewHorizontalScalingBuilder().WithMinReplicas(3).WithMaxReplicas(4).WithCPUTrigger(1).WithMemoryTrigger(1).Build())).
				BuildRD(),
			expectedReplicas: 0,
		},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			s.setupTest()
			sut := &oauthRedisResourceManager{test.rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
			err := sut.Sync(context.Background())
			s.Nil(err)
			deploys, _ := sut.kubeutil.KubeClient().AppsV1().Deployments(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: s.getAppNameSelector(appName)})
			s.Equal(test.expectedReplicas, *deploys.Items[0].Spec.Replicas)
		})
	}
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
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
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
	sut := &oauthRedisResourceManager{rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
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
	sut = &oauthRedisResourceManager{rd, rr, s.kubeUtil, zerolog.Nop(), s.cfg}
	err = sut.Sync(context.Background())
	s.Nil(err)
	actualDeploys, err = s.kubeClient.AppsV1().Deployments(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list deployments after OAuth config change")
	s.Len(actualDeploys.Items, 1)
	s.Equal(utils.GetAuxiliaryComponentDeploymentName(component1Name, v1.OAuthRedisAuxiliaryComponentSuffix), actualDeploys.Items[0].Name)
	s.Equal(s.cfg.Common.ExternalRegistryAuthSecret, actualDeploys.Items[0].Spec.Template.Spec.ImagePullSecrets[0].Name)
	actualServices, err = s.kubeClient.CoreV1().Services(envNs).List(context.Background(), metav1.ListOptions{})
	s.NoError(err, "failed to list services after OAuth config change")
	s.Len(actualServices.Items, 1)
	s.Equal(utils.GetAuxOAuthRedisServiceName(component1Name), actualServices.Items[0].Name)
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

func (s *OAuthRedisResourceManagerTestSuite) getObjectNames(items any) []string {
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
