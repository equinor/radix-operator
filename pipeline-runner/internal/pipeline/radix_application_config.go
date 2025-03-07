package pipeline

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/utils/configmap"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	"strings"

	pipelineDefaults "github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ProcessRadixAppConfig Load Radix config file to a ConfigMap and create RadixApplication
func (ctx *pipelineContext) ProcessRadixAppConfig() error {
	configFileContent, err := configmap.CreateFromRadixConfigFile(ctx.env)
	if err != nil {
		return fmt.Errorf("error reading the Radix config file %s: %v", ctx.GetPipelineInfo().GetRadixConfigFileName(), err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", ctx.GetPipelineInfo().GetRadixConfigFileName())

	ctx.radixApplication, err = application.CreateRadixApplication(context.TODO(), ctx.radixClient, ctx.env.GetAppName(), ctx.env.GetDNSConfig(), configFileContent)
	if err != nil {
		return err
	}
	log.Debug().Msg("Radix Application has been loaded")

	err = ctx.setTargetEnvironments()
	if err != nil {
		return err
	}
	err := ctx.preparePipelinesJob()
	if err != nil {
		return err
	}
	return ctx.createConfigMap(configFileContent, prepareBuildContext)
}

func (ctx *pipelineContext) createConfigMap(configFileContent string, prepareBuildContext *model.PrepareBuildContext) error {
	env := ctx.GetPipelineInfo()
	if prepareBuildContext == nil {
		prepareBuildContext = &model.PrepareBuildContext{}
	}
	buildContext, err := yaml.Marshal(prepareBuildContext)
	if err != nil {
		return err
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      env.GetRadixConfigMapName(),
			Namespace: env.GetAppNamespace(),
			Labels:    map[string]string{kube.RadixJobNameLabel: ctx.GetPipelineInfo().GetRadixPipelineJobName()},
		},
		Data: map[string]string{
			pipelineDefaults.PipelineConfigMapContent:      configFileContent,
			pipelineDefaults.PipelineConfigMapBuildContext: string(buildContext),
		},
	}
	if ctx.ownerReference != nil {
		cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ctx.ownerReference}
	}

	_, err = ctx.kubeClient.CoreV1().ConfigMaps(env.GetAppNamespace()).Create(context.Background(), &cm, metav1.CreateOptions{})

	if err != nil {
		return err
	}
	log.Debug().Msgf("Created ConfigMap %s", env.GetRadixConfigMapName())
	return nil
}

func (ctx *pipelineContext) setTargetEnvironments() error {
	if ctx.GetPipelineInfo().GetRadixPipelineType() == v1.ApplyConfig {
		return nil
	}
	log.Debug().Msg("Set target environment")
	if ctx.GetPipelineInfo().GetRadixPipelineType() == v1.Promote {
		return ctx.setTargetEnvironmentsForPromote()
	}
	if ctx.GetPipelineInfo().GetRadixPipelineType() == v1.Deploy {
		return ctx.setTargetEnvironmentsForDeploy()
	}
	targetEnvironments := applicationconfig.GetTargetEnvironments(ctx.pipelineInfo.PipelineArguments.Branch, ctx.radixApplication)
	ctx.targetEnvironments = make(map[string]bool)
	deployToEnvironment := ctx.env.GetRadixDeployToEnvironment()
	for _, envName := range targetEnvironments {
		if len(deployToEnvironment) == 0 || deployToEnvironment == envName {
			ctx.targetEnvironments[envName] = true
		}
	}
	if len(ctx.targetEnvironments) > 0 {
		log.Info().Msgf("Environment(s) %v are mapped to the branch %s.", getEnvironmentList(ctx.targetEnvironments), ctx.pipelineInfo.PipelineArguments.Branch)
	} else {
		log.Info().Msgf("No environments are mapped to the branch %s.", ctx.pipelineInfo.PipelineArguments.Branch)
	}
	log.Info().Msgf("Pipeline type: %s", ctx.env.GetRadixPipelineType())
	return nil
}

func (ctx *pipelineContext) setTargetEnvironmentsForPromote() error {
	var errs []error
	if len(ctx.env.GetRadixPromoteDeployment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote deployment name"))
	}
	if len(ctx.env.GetRadixPromoteFromEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote source environment name"))
	}
	if len(ctx.env.GetRadixDeployToEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote target environment name"))
	}
	if len(errs) > 0 {
		log.Info().Msg("Pipeline type: promote")
		return errors.Join(errs...)
	}
	ctx.targetEnvironments = map[string]bool{ctx.env.GetRadixDeployToEnvironment(): true} // run Tekton pipelines for the promote target environment
	log.Info().Msgf("promote the deployment %s from the environment %s to %s", ctx.env.GetRadixPromoteDeployment(), ctx.env.GetRadixPromoteFromEnvironment(), ctx.env.GetRadixDeployToEnvironment())
	return nil
}

func (ctx *pipelineContext) setTargetEnvironmentsForDeploy() error {
	targetEnvironment := ctx.env.GetRadixDeployToEnvironment()
	if len(targetEnvironment) == 0 {
		return fmt.Errorf("no target environment is specified for the deploy pipeline")
	}
	ctx.targetEnvironments = map[string]bool{targetEnvironment: true}
	log.Info().Msgf("Target environment: %v", targetEnvironment)
	log.Info().Msgf("Pipeline type: %s", ctx.env.GetRadixPipelineType())
	return nil
}

func getEnvironmentList(environmentNameMap map[string]bool) string {
	var envNames []string
	for envName := range environmentNameMap {
		envNames = append(envNames, envName)
	}
	return strings.Join(envNames, ", ")
}
