package test

import (
	"context"

	"github.com/equinor/radix-operator/pipeline-runner/internal/hash"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	yamlk8s "sigs.k8s.io/yaml"
)

func CreatePreparePipelineConfigMapResponse(kubeClient kubernetes.Interface, configMapName, appName string, ra *radixv1.RadixApplication, buildCtx *model.PrepareBuildContext) error {
	raBytes, err := yamlk8s.Marshal(ra)
	if err != nil {
		return err
	}
	data := map[string]string{
		pipelineDefaults.PipelineConfigMapContent: string(raBytes),
	}

	if buildCtx != nil {
		buildCtxBytes, err := yaml.Marshal(buildCtx)
		if err != nil {
			return err
		}
		data[pipelineDefaults.PipelineConfigMapBuildContext] = string(buildCtxBytes)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data:       data,
	}
	_, err = kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(context.Background(), cm, metav1.CreateOptions{})
	return err
}

func CreateGitInfoConfigMapResponse(kubeClient kubernetes.Interface, configMapName, appName, gitHash, gitTags string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data: map[string]string{
			defaults.RadixGitCommitHashKey: gitHash,
			defaults.RadixGitTagsKey:       gitTags,
		},
	}
	_, err := kubeClient.CoreV1().ConfigMaps(utils.GetAppNamespace(appName)).Create(context.Background(), cm, metav1.CreateOptions{})
	return err
}

func CreateBuildSecret(kubeClient kubernetes.Interface, appName string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.BuildSecretsName},
		Data:       data,
	}

	_, err := kubeClient.CoreV1().Secrets(utils.GetAppNamespace(appName)).Create(context.Background(), secret, metav1.CreateOptions{})
	return err
}

func GetRadixApplicationHash(ra *radixv1.RadixApplication) string {
	if ra == nil {
		hash, _ := hash.ToHashString(hash.SHA256, "0nXSg9l6EUepshGFmolpgV3elB0m8Mv7")
		return hash
	}
	hash, _ := hash.ToHashString(hash.SHA256, ra.Spec)
	return hash
}

func GetBuildSecretHash(secret *corev1.Secret) string {
	if secret == nil {
		hash, _ := hash.ToHashString(hash.SHA256, "34Wd68DsJRUzrHp2f63o3U5hUD6zl8Tj")
		return hash
	}
	hash, _ := hash.ToHashString(hash.SHA256, secret.Data)
	return hash
}
