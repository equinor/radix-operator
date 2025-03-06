package configmap

import (
	"context"
	"fmt"
	"os"

	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-tekton/pkg/models/env"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateFromRadixConfigFile Creates a configmap by name from file and returns as content
func CreateFromRadixConfigFile(env env.Env) (string, error) {
	content, err := os.ReadFile(env.GetRadixConfigFileName())
	if err != nil {
		return "", fmt.Errorf("could not find or read config yaml file \"%s\"", env.GetRadixConfigFileName())
	}
	return string(content), nil
}

// CreateGitConfigFromGitRepository create configmap with git repository information
func CreateGitConfigFromGitRepository(env env.Env, kubeClient kubernetes.Interface, targetCommitHash, gitTags string, ownerReference *metav1.OwnerReference) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.GetGitConfigMapName(),
			Namespace: env.GetAppNamespace(),
			Labels:    map[string]string{kube.RadixJobNameLabel: env.GetRadixPipelineJobName()},
		},
		Data: map[string]string{
			defaults.RadixGitCommitHashKey: targetCommitHash,
			defaults.RadixGitTagsKey:       gitTags,
		},
	}

	if ownerReference != nil {
		cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerReference}
	}
	_, err := kubeClient.CoreV1().ConfigMaps(env.GetAppNamespace()).Create(
		context.Background(),
		&cm,
		metav1.CreateOptions{})

	if err != nil {
		return err
	}
	log.Debug().Msgf("Created ConfigMap %s", env.GetGitConfigMapName())
	return nil
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
