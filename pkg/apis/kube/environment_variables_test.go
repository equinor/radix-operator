package kube_test

import (
	"context"
	"strings"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	_ "github.com/equinor/radix-operator/pkg/apis/test"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedav2 "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	secretProviderClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type EnvironmentVariablesTestEnv struct {
	kubeclient           kubernetes.Interface
	radixclient          radixclient.Interface
	kedaClient           kedav2.Interface
	secretproviderclient secretProviderClient.Interface
	prometheusclient     prometheusclient.Interface
	kubeUtil             *kube.Kube
}

func getEnvironmentVariablesTestEnv() EnvironmentVariablesTestEnv {
	testEnv := EnvironmentVariablesTestEnv{
		kubeclient:           kubefake.NewSimpleClientset(),
		radixclient:          radixfake.NewSimpleClientset(),
		kedaClient:           kedafake.NewSimpleClientset(),
		secretproviderclient: secretproviderfake.NewSimpleClientset(),
		prometheusclient:     prometheusfake.NewSimpleClientset(),
	}
	kubeUtil, _ := kube.New(testEnv.kubeclient, testEnv.radixclient, testEnv.kedaClient, testEnv.secretproviderclient)
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

		configMap, err := kube.GetEnvVarsMetadataFromConfigMap(context.Background(), &testConfigMap)
		assert.NoError(t, err)
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

		metadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(context.Background(), &testConfigMap)

		assert.NoError(t, err)
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

		metadataMap, err := kube.GetEnvVarsMetadataFromConfigMap(context.Background(), &testConfigMap)

		assert.NotNil(t, err)
		assert.Nil(t, metadataMap)
		assert.Contains(t, err.Error(), "invalid")
	})
}

func Test_GetEnvVarsMetadataConfigMapAndMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	createEnvVarConfigMapFunc := func(testEnv EnvironmentVariablesTestEnv) {
		err := createConfigMap(testEnv.kubeUtil, namespace, &corev1.ConfigMap{
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
		require.NoError(t, err)

	}
	createEnvVarMetadataConfigMapFunc := func(testEnv EnvironmentVariablesTestEnv) {
		err := createConfigMap(
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
		require.NoError(t, err)

	}
	t.Run("Get existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		createEnvVarConfigMapFunc(testEnv)
		createEnvVarMetadataConfigMapFunc(testEnv)

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, componentName)

		assert.NoError(t, err)
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

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, componentName)

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

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, componentName)

		assert.NotNil(t, err)
		assert.Equal(t, "configmaps \"env-vars-metadata-comp1\" not found", err.Error())
		assert.Nil(t, envVarsConfigMap)
		assert.Nil(t, envVarsMetadataConfigMap)
		assert.Nil(t, metadataMap)
	})
}

func Test_SetEnvVarsMetadataMapToConfigMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	t.Run("Set map", func(t *testing.T) {
		t.Parallel()
		currentMetadataConfigMap := corev1.ConfigMap{
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
		}

		err := kube.SetEnvVarsMetadataMapToConfigMap(&currentMetadataConfigMap, map[string]kube.EnvVarMetadata{
			"VAR1": {RadixConfigValue: "val1changed"},
			"VAR2": {RadixConfigValue: "added"},
			// VAR3: removed
		})
		require.NoError(t, err)

		assert.NotNil(t, currentMetadataConfigMap.Data)
		assert.NotNil(t, currentMetadataConfigMap.Data["metadata"])
		metadataText := currentMetadataConfigMap.Data["metadata"]
		metadataText = strings.ReplaceAll(metadataText, " ", "")
		metadataText = strings.ReplaceAll(metadataText, "\n", "")
		metadataText = strings.ReplaceAll(metadataText, "\t", "")
		assert.True(t, strings.Contains(metadataText, "VAR1"))
		assert.True(t, strings.Contains(metadataText, "\"RadixConfigValue\":\"val1changed\""))
		assert.True(t, strings.Contains(metadataText, "VAR2"))
		assert.True(t, strings.Contains(metadataText, "\"RadixConfigValue\":\"added\""))
		assert.False(t, strings.Contains(metadataText, "VAR3"))
	})
}

func Test_ApplyEnvVarsMetadataConfigMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	name := "env-vars-metadata-" + componentName
	currentMetadataConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
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
	}
	metadata := map[string]kube.EnvVarMetadata{
		"VAR1": {RadixConfigValue: "val1changed"},
		"VAR2": {RadixConfigValue: "added"},
		// VAR3: removed
	}

	t.Run("Save changes", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		_, err := testEnv.kubeclient.CoreV1().ConfigMaps(namespace).Create(context.Background(), &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			}}, metav1.CreateOptions{})
		require.NoError(t, err)

		err = testEnv.kubeUtil.ApplyEnvVarsMetadataConfigMap(context.Background(), namespace, &currentMetadataConfigMap, metadata)
		assert.NoError(t, err)

		configMap, err := testEnv.kubeclient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, configMap)
		assert.NotNil(t, configMap.Data)
		assert.NotNil(t, configMap.Data["metadata"])
		metadataText := configMap.Data["metadata"]
		metadataText = strings.ReplaceAll(metadataText, " ", "")
		metadataText = strings.ReplaceAll(metadataText, "\n", "")
		metadataText = strings.ReplaceAll(metadataText, "\t", "")
		assert.True(t, strings.Contains(metadataText, "VAR1"))
		assert.True(t, strings.Contains(metadataText, "\"RadixConfigValue\":\"val1changed\""))
		assert.True(t, strings.Contains(metadataText, "VAR2"))
		assert.True(t, strings.Contains(metadataText, "\"RadixConfigValue\":\"added\""))
		assert.False(t, strings.Contains(metadataText, "VAR3"))
	})

	t.Run("Fail to save to non-existing config-map", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		err := testEnv.kubeUtil.ApplyEnvVarsMetadataConfigMap(context.Background(), namespace, &currentMetadataConfigMap, metadata)
		assert.NotNil(t, err)
		assert.Equal(t, "failed to patch config-map object: configmaps \"env-vars-metadata-comp1\" not found", err.Error())
	})
}

func Test_GetEnvVarsConfigMapAndMetadataMap(t *testing.T) {
	namespace := "some-namespace"
	componentName := "comp1"
	t.Run("Get existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		err := createConfigMap(testEnv.kubeUtil, namespace, &corev1.ConfigMap{
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
		require.NoError(t, err)

		err = createConfigMap(
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
		require.NoError(t, err)

		envVarsConfigMap, envVarsMetadataConfigMap, metadataMap, err := testEnv.kubeUtil.GetEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, componentName)
		assert.NoError(t, err)
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

func Test_GetOrCreateEnvVarsConfigMapAndMetadataMap(t *testing.T) {
	appName := "some-app"
	namespace := "some-add-dev"
	componentName := "comp1"

	t.Run("Create new, when does not exists", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()

		envVarsConfigMap, envVarsMetadataConfigMap, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, appName, componentName)

		assert.NoError(t, err)
		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.NotNil(t, envVarsMetadataConfigMap)
		assert.NotNil(t, envVarsMetadataConfigMap.Data)
		assert.Equal(t, "", envVarsMetadataConfigMap.Data["metadata"])
	})

	t.Run("Get existing", func(t *testing.T) {
		t.Parallel()
		testEnv := getEnvironmentVariablesTestEnv()
		err := createConfigMap(testEnv.kubeUtil, namespace, &corev1.ConfigMap{
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
		require.NoError(t, err)

		err = createConfigMap(
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
		require.NoError(t, err)

		envVarsConfigMap, envVarsMetadataConfigMap, err := testEnv.kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(context.Background(), namespace, appName, componentName)

		assert.NoError(t, err)
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
	})
}

func Test_BuildRadixConfigEnvVarsConfigMap(t *testing.T) {
	appName := "some-app"
	componentName := "comp1"
	name := "env-vars-" + componentName
	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		envVarsConfigMap := kube.BuildRadixConfigEnvVarsConfigMap(appName, componentName)

		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Equal(t, name, envVarsConfigMap.ObjectMeta.Name)
		assert.Equal(t, appName, envVarsConfigMap.ObjectMeta.Labels[kube.RadixAppLabel])
		assert.Equal(t, componentName, envVarsConfigMap.ObjectMeta.Labels[kube.RadixComponentLabel])
		assert.Equal(t, string(kube.EnvVarsConfigMap), envVarsConfigMap.ObjectMeta.Labels[kube.RadixConfigMapTypeLabel])
	})
}

func Test_BuildRadixConfigEnvVarsMetadataConfigMap(t *testing.T) {
	appName := "some-app"
	componentName := "comp1"
	name := "env-vars-metadata-" + componentName
	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		envVarsConfigMap := kube.BuildRadixConfigEnvVarsMetadataConfigMap(appName, componentName)

		assert.NotNil(t, envVarsConfigMap)
		assert.NotNil(t, envVarsConfigMap.Data)
		assert.Equal(t, name, envVarsConfigMap.ObjectMeta.Name)
		assert.Equal(t, appName, envVarsConfigMap.ObjectMeta.Labels[kube.RadixAppLabel])
		assert.Equal(t, componentName, envVarsConfigMap.ObjectMeta.Labels[kube.RadixComponentLabel])
		assert.Equal(t, string(kube.EnvVarsMetadataConfigMap), envVarsConfigMap.ObjectMeta.Labels[kube.RadixConfigMapTypeLabel])
	})
}

func createConfigMap(kubeUtil *kube.Kube, namespace string, configMap *corev1.ConfigMap) error {
	_, err := kubeUtil.KubeClient().CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	return err
}
