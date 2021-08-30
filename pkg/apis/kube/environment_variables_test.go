package kube

import (
	"context"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"testing"
)

type EnvironmentVariablesSuite struct {
	suite.Suite
}

func TestEnvironmentVariablesSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentVariablesSuite))
}

type EnvironmentVariablesTestEnv struct {
	kubeclient       kubernetes.Interface
	radixclient      radixclient.Interface
	prometheusclient prometheusclient.Interface
	kubeUtil         *Kube
}

func getEnvironmentVariablesTestEnv() ConfigMapTestEnv {
	testEnv := ConfigMapTestEnv{
		kubeclient:       kubefake.NewSimpleClientset(),
		radixclient:      radix.NewSimpleClientset(),
		prometheusclient: prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := New(testEnv.kubeclient, testEnv.radixclient)
	testEnv.kubeUtil = kubeUtil
	return testEnv
}

func Test_GetEnvVarsMetadataFromConfigMap(t *testing.T) {
	name := "some name"
	t.Run("Get metadata from ConfigMap with nil data", func(t *testing.T) {
		t.Parallel()

		testConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data:       nil,
		}

		configMap, err := GetEnvVarsMetadataFromConfigMap(&testConfigMap)
		assert.Nil(t, err)
		assert.NotNil(t, configMap)
		assert.Len(t, configMap, 0)

	})
	t.Run("Get metadata from ConfigMap with valid metadata", func(t *testing.T) {
		t.Parallel()

		testConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data: map[string]string{"metadata": `
											{
												"VAR1":{"RadixConfigValue":"val1"},
												"VAR2":{"RadixConfigValue":"val2"},
												"VAR3":{"RadixConfigValue":""}
											}
											`},
		}

		metadataMap, err := GetEnvVarsMetadataFromConfigMap(&testConfigMap)

		assert.Nil(t, err)
		assert.NotNil(t, metadataMap)
		assert.Len(t, metadataMap, 3)
		assert.NotNil(t, metadataMap["VAR1"])
		assert.Equal(t, "val1", metadataMap["VAR1"].RadixConfigValue)
		assert.NotNil(t, metadataMap["VAR2"])
		assert.Equal(t, "val2", metadataMap["VAR2"].RadixConfigValue)
		assert.NotNil(t, metadataMap["VAR3"])
		assert.Equal(t, "", metadataMap["VAR3"].RadixConfigValue)
	})
	t.Run("Get metadata from ConfigMap with invalid metadata", func(t *testing.T) {
		t.Parallel()

		testConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Data:       map[string]string{"metadata": "invalid-value"},
		}

		metadataMap, err := GetEnvVarsMetadataFromConfigMap(&testConfigMap)

		assert.NotNil(t, err)
		assert.Nil(t, metadataMap)
		assert.Contains(t, err.Error(), "invalid")
	})
}

func Test_GetEnvVarsMetadataConfigMapAndMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	createEnvVarConfigMapFunc := func(testEnv ConfigMapTestEnv) {
		createConfigMap(testEnv.kubeUtil, namespace, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "env-vars-" + componentName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"VAR1": "val1changed",
				"VAR2": "val2",
				"VAR3": "setVal3",
			},
		})
	}
	createEnvVarMetadataConfigMapFunc := func(testEnv ConfigMapTestEnv) {
		createConfigMap(
			testEnv.kubeUtil,
			namespace,
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "env-vars-metadata-" + componentName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"metadata": `
							{
								"VAR1":{"RadixConfigValue":"val1"},
								"VAR3":{"RadixConfigValue":""}
							}
							`,
				},
			},
		)
	}
	t.Run("Get existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		createEnvVarConfigMapFunc(testEnv)
		createEnvVarMetadataConfigMapFunc(testEnv)

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)

		assert.Nil(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Len(t, envVarsConfigMap.Data, 3)
		assert.NotNil(t, envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "val1changed", envVarsConfigMap.Data["VAR1"])
		assert.NotNil(t, envVarsConfigMap.Data["VAR2"])
		assert.Equal(t, "val2", envVarsConfigMap.Data["VAR2"])
		assert.NotNil(t, envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "setVal3", envVarsConfigMap.Data["VAR3"])
		assert.NotNil(t, envVarsMetadataConfigMap)
		assert.NotNil(t, envVarsMetadataConfigMap.Data)
		assert.NotNil(t, envVarsMetadataConfigMap.Data["metadata"])
		assert.NotNil(t, metadataMap)
		assert.Len(t, metadataMap, 2)
		assert.NotNil(t, metadataMap["VAR1"])
		assert.Equal(t, "val1", metadataMap["VAR1"].RadixConfigValue)
		assert.NotNil(t, metadataMap["VAR3"])
		assert.Equal(t, "", metadataMap["VAR3"].RadixConfigValue)
	})
	t.Run("Failed get when env-vars configmap not existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		createEnvVarMetadataConfigMapFunc(testEnv)

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)

		assert.NotNil(t, err)
		assert.Equal(t, "configmaps \"env-vars-comp1\" not found", err.Error())
		assert.Nil(t, envVarsConfigMap)
		assert.Nil(t, envVarsMetadataConfigMap)
		assert.Nil(t, metadataMap)
	})
	t.Run("Failed get when env-vars configmap not existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		createEnvVarConfigMapFunc(testEnv)

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)

		assert.NotNil(t, err)
		assert.Equal(t, "configmaps \"env-vars-metadata-comp1\" not found", err.Error())
		assert.Nil(t, envVarsConfigMap)
		assert.Nil(t, envVarsMetadataConfigMap)
		assert.Nil(t, metadataMap)
	})
}

func Test_GetEnvVarsConfigMapAndMetadataMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	t.Run("Get existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		createConfigMap(testEnv.kubeUtil, namespace, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "env-vars-" + componentName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"VAR1": "val1changed",
				"VAR2": "val2",
				"VAR3": "setVal3",
			},
		})
		createConfigMap(
			testEnv.kubeUtil,
			namespace,
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "env-vars-metadata-" + componentName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"metadata": `
							{
								"VAR1":{"RadixConfigValue":"val1"},
								"VAR3":{"RadixConfigValue":""}
							}
							`,
				},
			},
		)
		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(namespace, componentName)
		assert.Nil(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Len(t, envVarsConfigMap.Data, 3)
		assert.NotNil(t, envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "val1changed", envVarsConfigMap.Data["VAR1"])
		assert.NotNil(t, envVarsConfigMap.Data["VAR2"])
		assert.Equal(t, "val2", envVarsConfigMap.Data["VAR2"])
		assert.NotNil(t, envVarsConfigMap.Data["VAR1"])
		assert.Equal(t, "setVal3", envVarsConfigMap.Data["VAR3"])
		assert.NotNil(t, envVarsMetadataConfigMap)
		assert.NotNil(t, envVarsMetadataConfigMap.Data)
		assert.NotNil(t, envVarsMetadataConfigMap.Data["metadata"])
		assert.NotNil(t, metadataMap)
		assert.Len(t, metadataMap, 2)
		assert.NotNil(t, metadataMap["VAR1"])
		assert.Equal(t, "val1", metadataMap["VAR1"].RadixConfigValue)
		assert.NotNil(t, metadataMap["VAR3"])
		assert.Equal(t, "", metadataMap["VAR3"].RadixConfigValue)
	})
}

func createConfigMap(kubeUtil *Kube, namespace string, configMap *corev1.ConfigMap) {
	kubeUtil.kubeClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
}
