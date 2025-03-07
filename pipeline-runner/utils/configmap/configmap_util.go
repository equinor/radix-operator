package configmap

import (
	"context"
	"fmt"
	"os"

	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ReadFromRadixConfigFile Reads radixconfig content from a file
func ReadFromRadixConfigFile(radixConfigFileName string) (string, error) {
	content, err := os.ReadFile(radixConfigFileName)
	if err != nil {
		return "", fmt.Errorf("could not find or read config yaml file \"%s\"", radixConfigFileName)
	}
	return string(content), nil
}

// GetRadixConfigFromConfigMap Get Radix config from the ConfigMap
func GetRadixConfigFromConfigMap(kubeClient kubernetes.Interface, namespace, configMapName string) (string, error) {
	configMap, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", fmt.Errorf("no ConfigMap %s", configMapName)
		}
		return "", err
	}
	if configMap.Data == nil {
		return "", getNoRadixConfigInConfigMap(configMapName)
	}
	content, ok := configMap.Data[pipelineDefaults.PipelineConfigMapContent]
	if !ok {
		return "", getNoRadixConfigInConfigMap(configMapName)
	}
	return content, nil
}

func getNoRadixConfigInConfigMap(configMapName string) error {
	return fmt.Errorf("no RadixConfig in the ConfigMap %s", configMapName)
}
