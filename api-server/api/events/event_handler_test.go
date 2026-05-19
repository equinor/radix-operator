package events

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	eventModels "github.com/equinor/radix-operator/api-server/api/events/models"
	"github.com/equinor/radix-operator/api-server/models"
	operatorutils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const (
	appName1          = "app1"
	appName2          = "app2"
	envName1          = "env1"
	ev1               = "ev1"
	ev2               = "ev2"
	ev3               = "ev3"
	uid1              = "d1bf3ab3-0693-4291-a559-96a12ace9f33"
	uid2              = "2dcc9cf7-086d-49a0-abe1-ea3610594eb2"
	uid3              = "0c43a075-d174-479e-96b1-70183d67464c"
	component1        = "server1"
	component2        = "server2"
	component3        = "server3"
	deploy1           = component1
	deploy2           = component2
	deploy3           = component3
	replicaSetServer1 = deploy1 + "-4bf67cf976"
	replicaSetServer2 = deploy2 + "-795977897d"
	replicaSetServer3 = deploy3 + "-5c97b4c698"
	podServer1        = replicaSetServer1 + "-9v333"
	podServer2        = replicaSetServer2 + "-m2sw8"
	podServer3        = replicaSetServer3 + "-6x4cg"
)

type scenario struct {
	name               string
	existingEventProps []eventProps
	expectedEvents     []eventModels.Event
}

type eventProps struct {
	name          string
	appName       string
	envName       string
	componentName string
	podName       string
	eventType     string
	objectName    string
	objectUid     string
	objectKind    string
}

func setupTest() (*kubefake.Clientset, *radixfake.Clientset) {
	kubeClient := kubefake.NewSimpleClientset()   //nolint:staticcheck
	radixClient := radixfake.NewSimpleClientset() //nolint:staticcheck
	return kubeClient, radixClient
}

func Test_EventHandler_Init(t *testing.T) {
	kubeClient, radixClient := setupTest()
	accounts := models.Accounts{
		UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
		ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}
	eh := Init(accounts).(*eventHandler)
	assert.NotNil(t, eh)
	assert.Equal(t, kubeClient, eh.accounts.UserAccount.Client)
}

func Test_EventHandler_NoEventsWhenThereIsNoRadixApplication(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()
	accounts := models.Accounts{
		UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
		ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

	createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)

	eventHandler := Init(accounts)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.NotNil(t, err)
	assert.Len(t, events, 0)
}

func Test_EventHandler_NoEventsWhenThereIsNoRadixEnvironment(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	kubeClient, radixClient := setupTest()
	accounts := models.Accounts{
		UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
		ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

	createRadixApp(t, kubeClient, radixClient, appName, envName)
	err := radixClient.RadixV1().RadixEnvironments().Delete(context.Background(), fmt.Sprintf("%s-%s", appName, envName), metav1.DeleteOptions{})
	require.NoError(t, err)
	createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)

	eventHandler := Init(accounts)
	events, err := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
	assert.NotNil(t, err)
	assert.Len(t, events, 0)
}

func Test_EventHandler_GetEvents_PodState(t *testing.T) {
	appName, envName := "app1", "env1"
	envNamespace := operatorutils.GetEnvironmentNamespace(appName, envName)

	t.Run("ObjectState is nil for normal event type", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		accounts := models.Accounts{
			UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
			ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

		createRadixApp(t, kubeClient, radixClient, appName, envName)
		_, err := createKubernetesPod(kubeClient, podServer1, appName, envName, true, true, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)
		require.NoError(t, err)
		eventHandler := Init(accounts)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})

	t.Run("ObjectState has Pod state for warning event type", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		accounts := models.Accounts{
			UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
			ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

		createRadixApp(t, kubeClient, radixClient, appName, envName)
		_, err := createKubernetesPod(kubeClient, podServer1, appName, envName, true, false, 0, uid1)
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeWarning, podServer1, k8sKindPod, uid1)
		require.NoError(t, err)
		eventHandler := Init(accounts)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.NotNil(t, events[0].InvolvedObjectState)
		assert.NotNil(t, events[0].InvolvedObjectState.Pod)
	})

	t.Run("ObjectState is nil for warning event type when pod not exist", func(t *testing.T) {
		kubeClient, radixClient := setupTest()
		accounts := models.Accounts{
			UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
			ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

		createRadixApp(t, kubeClient, radixClient, appName, envName)
		createKubernetesEvent(t, kubeClient, envNamespace, ev1, k8sEventTypeNormal, podServer1, k8sKindPod, uid1)
		eventHandler := Init(accounts)
		events, _ := eventHandler.GetEnvironmentEvents(context.Background(), appName, envName)
		assert.Len(t, events, 1)
		assert.Nil(t, events[0].InvolvedObjectState)
	})
}

func Test_EventHandler_GetEnvironmentEvents(t *testing.T) {
	envNamespace := operatorutils.GetEnvironmentNamespace(appName1, envName1)

	scenarios := []scenario{
		{
			name: "Pod events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: podServer1, objectKind: k8sKindPod, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: podServer2, objectKind: k8sKindPod, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, eventType: k8sEventTypeNormal, objectName: podServer3, objectKind: k8sKindPod, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: podServer1, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: podServer2, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Deploy events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: deploy1, objectKind: k8sKindDeployment, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: deploy2, objectKind: k8sKindDeployment, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, eventType: k8sEventTypeNormal, objectName: deploy3, objectKind: k8sKindDeployment, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: deploy1, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: deploy2, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "ReplicaSet events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: replicaSetServer1, objectKind: k8sKindReplicaSet, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, eventType: k8sEventTypeNormal, objectName: replicaSetServer2, objectKind: k8sKindReplicaSet, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, eventType: k8sEventTypeNormal, objectName: replicaSetServer3, objectKind: k8sKindReplicaSet, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: replicaSetServer1, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
				{InvolvedObjectName: replicaSetServer2, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			eventHandler, _ := setupTestEnvForHandler(t, ts)
			actualEvents, err := eventHandler.GetEnvironmentEvents(context.Background(), appName1, envName1)
			assert.Nil(t, err)
			assertEvents(t, ts.expectedEvents, actualEvents)
		})
	}
}

func Test_EventHandler_GetComponentEvents(t *testing.T) {
	envNamespace := operatorutils.GetEnvironmentNamespace(appName1, envName1)

	scenarios := []scenario{
		{
			name: "Pod events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: podServer1, objectKind: k8sKindPod, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, eventType: k8sEventTypeNormal, objectName: podServer2, objectKind: k8sKindPod, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: podServer3, objectKind: k8sKindPod, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: podServer1, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Deploy events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: deploy1, objectKind: k8sKindDeployment, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, eventType: k8sEventTypeNormal, objectName: deploy2, objectKind: k8sKindDeployment, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: deploy3, objectKind: k8sKindDeployment, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: deploy1, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "ReplicaSet events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: replicaSetServer1, objectKind: k8sKindReplicaSet, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, eventType: k8sEventTypeNormal, objectName: replicaSetServer2, objectKind: k8sKindReplicaSet, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, eventType: k8sEventTypeNormal, objectName: replicaSetServer3, objectKind: k8sKindReplicaSet, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: replicaSetServer1, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			eventHandler, radixClient := setupTestEnvForHandler(t, ts)
			createActiveRadixDeployments(t, ts, radixClient)

			actualEvents, err := eventHandler.GetComponentEvents(context.Background(), appName1, envName1, component1)
			assert.Nil(t, err)
			assertEvents(t, ts.expectedEvents, actualEvents)
		})
	}
}

func Test_EventHandler_GetPodEvents(t *testing.T) {
	envNamespace := operatorutils.GetEnvironmentNamespace(appName1, envName1)

	scenarios := []scenario{
		{
			name: "Pod events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, podName: podServer1, eventType: k8sEventTypeNormal, objectName: podServer1, objectKind: k8sKindPod, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, podName: podServer2, eventType: k8sEventTypeNormal, objectName: podServer2, objectKind: k8sKindPod, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, podName: podServer3, eventType: k8sEventTypeNormal, objectName: podServer3, objectKind: k8sKindPod, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: podServer1, InvolvedObjectKind: k8sKindPod, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "Deploy events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, podName: podServer1, eventType: k8sEventTypeNormal, objectName: deploy1, objectKind: k8sKindDeployment, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, podName: podServer2, eventType: k8sEventTypeNormal, objectName: deploy2, objectKind: k8sKindDeployment, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, podName: podServer3, eventType: k8sEventTypeNormal, objectName: deploy3, objectKind: k8sKindDeployment, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: deploy1, InvolvedObjectKind: k8sKindDeployment, InvolvedObjectNamespace: envNamespace},
			},
		},
		{
			name: "ReplicaSet events",
			existingEventProps: []eventProps{
				{name: ev1, appName: appName1, envName: envName1, componentName: component1, podName: podServer1, eventType: k8sEventTypeNormal, objectName: replicaSetServer1, objectKind: k8sKindReplicaSet, objectUid: uid1},
				{name: ev2, appName: appName1, envName: envName1, componentName: component2, podName: podServer2, eventType: k8sEventTypeNormal, objectName: replicaSetServer2, objectKind: k8sKindReplicaSet, objectUid: uid2},
				{name: ev3, appName: appName2, envName: envName1, componentName: component1, podName: podServer3, eventType: k8sEventTypeNormal, objectName: replicaSetServer3, objectKind: k8sKindReplicaSet, objectUid: uid3},
			},
			expectedEvents: []eventModels.Event{
				{InvolvedObjectName: replicaSetServer1, InvolvedObjectKind: k8sKindReplicaSet, InvolvedObjectNamespace: envNamespace},
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			eventHandler, radixClient := setupTestEnvForHandler(t, ts)
			createActiveRadixDeployments(t, ts, radixClient)

			actualEvents, err := eventHandler.GetPodEvents(context.Background(), appName1, envName1, component1, podServer1)
			assert.Nil(t, err)
			assertEvents(t, ts.expectedEvents, actualEvents)
		})
	}
}

func createActiveRadixDeployments(t *testing.T, ts scenario, radixClient *radixfake.Clientset) {
	appEnvComponentMap := getAppEnvComponentMap(ts)
	for appName, envComponentNameMap := range appEnvComponentMap {
		for envName, componentNameMap := range envComponentNameMap {
			builder := operatorutils.
				NewDeploymentBuilder().
				WithAppName(appName).
				WithEnvironment(envName)
			for componentName := range componentNameMap {
				builder = builder.WithComponent(operatorutils.
					NewDeployComponentBuilder().
					WithName(componentName))
			}
			rd := builder.WithActiveFrom(time.Now()).BuildRD()
			_, err := radixClient.RadixV1().RadixDeployments(operatorutils.GetEnvironmentNamespace(appName, envName)).
				Create(context.Background(), rd, metav1.CreateOptions{})
			require.NoError(t, err)
		}
	}
}

func createRadixApplications(t *testing.T, ts scenario, kubeClient *kubefake.Clientset, radixClient *radixfake.Clientset) {
	appEnvComponentMap := getAppEnvComponentMap(ts)
	for appName, envComponentNameMap := range appEnvComponentMap {
		var envNames []string
		for envName := range envComponentNameMap {
			envNames = append(envNames, envName)
		}
		createRadixApp(t, kubeClient, radixClient, appName, envNames...)
	}
}

func getAppEnvComponentMap(ts scenario) map[string]map[string]map[string]struct{} {
	appEnvComponentMap := slice.Reduce(ts.existingEventProps, make(map[string]map[string]map[string]struct{}), func(acc map[string]map[string]map[string]struct{}, evProps eventProps) map[string]map[string]map[string]struct{} {
		if _, ok := acc[evProps.appName]; !ok {
			acc[evProps.appName] = make(map[string]map[string]struct{})
		}
		if _, ok := acc[evProps.appName][evProps.envName]; !ok {
			acc[evProps.appName][evProps.envName] = make(map[string]struct{})
		}
		acc[evProps.appName][evProps.envName][evProps.componentName] = struct{}{}
		return acc
	})
	return appEnvComponentMap
}

func getAppEnvPodsMap(ts scenario) map[string]map[string]map[string]string {
	appEnvComponentMap := slice.Reduce(ts.existingEventProps, make(map[string]map[string]map[string]string), func(acc map[string]map[string]map[string]string, evProps eventProps) map[string]map[string]map[string]string {
		if len(evProps.podName) == 0 {
			return nil
		}
		if _, ok := acc[evProps.appName]; !ok {
			acc[evProps.appName] = make(map[string]map[string]string)
		}
		if _, ok := acc[evProps.appName][evProps.envName]; !ok {
			acc[evProps.appName][evProps.envName] = make(map[string]string)
		}
		acc[evProps.appName][evProps.envName][evProps.podName] = evProps.objectUid
		return acc
	})
	return appEnvComponentMap
}

func assertEvents(t *testing.T, expectedEvents []eventModels.Event, actualEvents []*eventModels.Event) {
	if assert.Len(t, actualEvents, len(expectedEvents)) {
		for i := 0; i < len(expectedEvents); i++ {
			assert.Equal(t, expectedEvents[i].InvolvedObjectName, actualEvents[i].InvolvedObjectName)
			assert.Equal(t, expectedEvents[i].InvolvedObjectKind, actualEvents[i].InvolvedObjectKind)
			assert.Equal(t, expectedEvents[i].InvolvedObjectNamespace, actualEvents[i].InvolvedObjectNamespace)
		}
	}
}

func setupTestEnvForHandler(t *testing.T, ts scenario) (EventHandler, *radixfake.Clientset) {
	kubeClient, radixClient := setupTest()
	accounts := models.Accounts{
		UserAccount:    models.Account{Client: kubeClient, RadixClient: radixClient},
		ServiceAccount: models.Account{Client: kubeClient, RadixClient: radixClient}}

	createRadixApplications(t, ts, kubeClient, radixClient)
	for _, evProps := range ts.existingEventProps {
		createKubernetesEvent(t, kubeClient, operatorutils.GetEnvironmentNamespace(evProps.appName, evProps.envName), evProps.name, evProps.eventType, evProps.objectName, evProps.objectKind, evProps.objectUid)
	}
	appEnvPodsMap := getAppEnvPodsMap(ts)
	for appName, envPodNameMap := range appEnvPodsMap {
		for envName, podNameMap := range envPodNameMap {
			for podName, uid := range podNameMap {
				_, err := createKubernetesPod(kubeClient, podName, appName, envName, true, true, 0, uid)
				require.NoError(t, err)
			}
		}
	}

	eventHandler := Init(accounts)
	return eventHandler, radixClient
}

func createKubernetesEvent(t *testing.T, client *kubefake.Clientset, namespace, name, eventType, involvedObjectName, involvedObjectKind, uid string) {
	_, err := client.CoreV1().Events(namespace).CreateWithEventNamespaceWithContext(context.Background(), &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      involvedObjectKind,
			Name:      involvedObjectName,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Type: eventType,
	})
	require.NoError(t, err)
}

func createKubernetesPod(client *kubefake.Clientset, name, appName, envName string, started, ready bool, restartCount int32, uid string) (*corev1.Pod, error) {
	namespace := operatorutils.GetEnvironmentNamespace(appName, envName)
	return client.CoreV1().Pods(namespace).Create(context.Background(),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				UID:    types.UID(uid),
				Labels: radixlabels.ForApplicationName(appName),
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Started: &started, Ready: ready, RestartCount: restartCount},
				},
			},
		},
		metav1.CreateOptions{})
}

func createRadixApp(t *testing.T, kubeClient *kubefake.Clientset, radixClient *radixfake.Clientset, appName string, envName ...string) {
	builder := operatorutils.NewRadixApplicationBuilder().WithAppName(appName)
	for _, envName := range envName {
		builder = builder.WithEnvironment(envName, "")
		_, err := radixClient.RadixV1().RadixEnvironments().Create(context.Background(), operatorutils.NewEnvironmentBuilder().WithAppName(appName).WithEnvironmentName(envName).BuildRE(), metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorutils.GetEnvironmentNamespace(appName, envName)}}, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	_, err := radixClient.RadixV1().RadixApplications(operatorutils.GetAppNamespace(appName)).Create(context.Background(), builder.BuildRA(), metav1.CreateOptions{})
	require.NoError(t, err)
}
