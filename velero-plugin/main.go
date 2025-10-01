package main

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	internalconfig "github.com/equinor/radix-operator/velero-plugin/internal/config"
	"github.com/equinor/radix-operator/velero-plugin/internal/plugin/restore"
	"github.com/sirupsen/logrus"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	c := internalconfig.ParseConfig()
	logger := initLogger(c)
	kubeUtil, err := initKube()
	if err != nil {
		logger.Fatalf("unable to init Kube util: %s", err)
		return
	}

	logger.WithField("config", c).Info("Configuration")
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
			return &restore.RestoreBatchPlugin{
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
	kubeClient, radixClient, _, _, _, _, _ := utils.GetKubernetesClient(context.Background())
	return kube.New(kubeClient, radixClient, nil, nil)
}

func initLogger(cfg internalconfig.Config) logrus.FieldLogger {
	logLevelStr := cfg.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = logrus.InfoLevel.String()
	}
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = logrus.InfoLevel
		logrus.Infof("Invalid log level '%s', fallback to '%s'", logLevelStr, logLevel.String())
	}

	var formatter logrus.Formatter = &logrus.JSONFormatter{}
	if cfg.LogPrettyPrint {
		formatter = &logrus.TextFormatter{}
	}

	logger := logrus.New()
	logger.Formatter = formatter
	logger.Level = logLevel
	return logger
}
