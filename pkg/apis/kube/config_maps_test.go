package kube_test

import (
	"context"
	"testing"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type ConfigMapSuite struct {
	suite.Suite
}

func TestConfigMapSuite(t *testing.T) {
	suite.Run(t, new(ConfigMapSuite))
}

type ConfigMapTestEnv struct {
	kubeclient           kubernetes.Interface
	radixclient          radixclient.Interface
	kedaClient           kedav2.Interface
	secretproviderclient secretProviderClient.Interface
	prometheusclient     prometheusclient.Interface
	kubeUtil             *kube.Kube
}

func getConfigMapTestEnv() ConfigMapTestEnv {
	testEnv := ConfigMapTestEnv{
		kubeclient:           kubefake.NewSimpleClientset(),
		radixclient:          radix.NewSimpleClientset(),
		kedaClient:           kedafake.NewSimpleClientset(),
		secretproviderclient: secretproviderfake.NewSimpleClientset(),
		prometheusclient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeclient, testEnv.radixclient, testEnv.kedaClient, testEnv.secretproviderclient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

type ConfigMapScenario struct {
	cmName string
	labels map[string]string
}

func (suite *ConfigMapSuite) Test_CreateConfigMap() {
	labels := map[string]string{
		"label1": "label1Value",
		"label2": "label2Value",
	}
	suite.T().Run("Create CM", func(t *testing.T) {
		t.Parallel()
		testEnv := getConfigMapTestEnv()
		namespace := "some-namespace"
		name := "some-name"
		configMap, err := testEnv.kubeUtil.CreateConfigMap(context.Background(), namespace, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			}})
		assert.Nil(t, err)

		assert.True(t, radixutils.EqualStringMaps(labels, configMap.ObjectMeta.Labels))
		assert.Equal(t, name, configMap.ObjectMeta.Name)
		assert.Equal(t, namespace, configMap.ObjectMeta.Namespace)
	})
}

func (suite *ConfigMapSuite) Test_ConfigMapInCluster() {
	labels := map[string]string{
		"label1": "label1Value",
		"label2": "label2Value",
	}
	namespace := "some-namespace"
	scenarios := []ConfigMapScenario{
		{
			cmName: "cm1",
			labels: labels,
		},
		{
			cmName: "cm1",
			labels: map[string]string{},
		},
	}
	suite.T().Run("Create CM", func(t *testing.T) {
		t.Parallel()
		for _, scenario := range scenarios {
			testEnv := getConfigMapTestEnv()
			_, err := testEnv.kubeUtil.CreateConfigMap(context.Background(), namespace, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:   scenario.cmName,
					Labels: labels,
				}})
			assert.Nil(t, err)

			configMaps, err := testEnv.kubeclient.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{})
			assert.Nil(t, err)

			assert.Len(t, configMaps.Items, 1)
			savedConfigMap := configMaps.Items[0]
			assert.True(t, radixutils.EqualStringMaps(labels, savedConfigMap.ObjectMeta.Labels))
			assert.Equal(t, scenario.cmName, savedConfigMap.ObjectMeta.Name)
			assert.Equal(t, namespace, savedConfigMap.ObjectMeta.Namespace)
		}
	})
}

func Test_GetConfigMap(t *testing.T) {
	t.Run("Get config-map from client", func(t *testing.T) {
		t.Parallel()

		testEnv := getConfigMapTestEnv()
		namespace := "some-namespace"
		name := "some-name"
		testConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data:       map[string]string{"key1": "value1", "key2": "value2"},
		}
		_, _ = testEnv.kubeclient.CoreV1().ConfigMaps(namespace).Create(context.Background(), &testConfigMap, metav1.CreateOptions{})

		configMap, err := testEnv.kubeUtil.GetConfigMap(context.Background(), namespace, name)

		assert.Nil(t, err)
		assert.Equal(t, name, configMap.ObjectMeta.Name)
		assert.Equal(t, namespace, configMap.ObjectMeta.Namespace)
		assert.True(t, radixutils.EqualStringMaps(testConfigMap.Data, configMap.Data))
	})
}

func Test_ApplyConfigMap(t *testing.T) {
	namespace := "some-namespace"
	name := "some-name"
	currentConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       map[string]string{"key1": "value1", "key2": "value2"},
	}
	desiredConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       map[string]string{"key2": "value2changed", "key3": "value3"},
	}

	t.Run("Patch not existing config-map", func(t *testing.T) {
		t.Parallel()
		testEnv := getConfigMapTestEnv()

		err := testEnv.kubeUtil.ApplyConfigMap(context.Background(), namespace, &currentConfigMap, &desiredConfigMap)

		assert.NotNil(t, err)
		assert.Equal(t, "failed to patch config-map object: configmaps \"some-name\" not found", err.Error())
	})

	t.Run("Patch existing config-map", func(t *testing.T) {
		t.Parallel()
		testEnv := getConfigMapTestEnv()
		namespace := "some-namespace"
		name := "some-name"
		_, _ = testEnv.kubeclient.CoreV1().ConfigMaps(namespace).Create(context.Background(), &currentConfigMap, metav1.CreateOptions{})

		err := testEnv.kubeUtil.ApplyConfigMap(context.Background(), namespace, &currentConfigMap, &desiredConfigMap)
		require.NoError(t, err)

		configMap, err := testEnv.kubeUtil.GetConfigMap(context.Background(), namespace, name)
		require.NoError(t, err)
		assert.Equal(t, name, configMap.ObjectMeta.Name)
		assert.Equal(t, namespace, configMap.ObjectMeta.Namespace)
		assert.True(t, radixutils.EqualStringMaps(desiredConfigMap.Data, configMap.Data))
	})
}
