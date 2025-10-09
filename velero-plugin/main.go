package main

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/velero-plugin/internal/plugin/restore"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	kubeUtil, err := initKube()
	if err != nil {
		logrus.Fatalf("unable to init Kube util: %s", err)
	}

	veleroplugin.NewServer().
		RegisterRestoreItemAction("equinor.com/restore-application-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreRadixApplicationPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-deployment-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreRadixDeploymentPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-job-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreRadixJobPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-alert-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreAlertPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-environment-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreRadixEnvironmentPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-batch-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreRadixBatchPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-secret-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreSecretPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		RegisterRestoreItemAction("equinor.com/restore-configmap-plugin", func(logger logrus.FieldLogger) (any, error) {
			return &restore.RestoreConfigMapPlugin{
				Log:  logger,
				Kube: kubeUtil,
			}, nil
		}).
		Serve()
}

func initKube() (*kube.Kube, error) {
	zerolog.DefaultContextLogger = &log.Logger
	kubeClient, radixClient, _, _, _, _, _ := utils.GetKubernetesClient(context.Background())
	return kube.New(kubeClient, radixClient, nil, nil)
}
