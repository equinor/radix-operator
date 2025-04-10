package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

// GetPipelineTargetEnvironments Get target environments for the pipeline
func GetPipelineTargetEnvironments(ctx context.Context, pipelineInfo *model.PipelineInfo) ([]string, []string, error) {
	log.Ctx(ctx).Debug().Msg("Set target environment")
	switch pipelineInfo.GetRadixPipelineType() {
	case v1.ApplyConfig:
		return nil, nil, nil
	case v1.Promote:
		environmentsForPromote, err := setTargetEnvironmentsForPromote(ctx, pipelineInfo)
		return environmentsForPromote, nil, err
	case v1.Deploy:
		environmentsForDeploy, err := setTargetEnvironmentsForDeploy(ctx, pipelineInfo)
		return environmentsForDeploy, nil, err
	}
	var targetEnvironments []string
	deployToEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	environments, ignoredForWebhookEnvs := applicationconfig.GetTargetEnvironments(pipelineInfo.GetBranch(), pipelineInfo.GetRadixApplication(), pipelineInfo.PipelineArguments.TriggeredFromWebhook)
	for _, envName := range environments {
		if len(deployToEnvironment) == 0 || deployToEnvironment == envName {
			targetEnvironments = append(targetEnvironments, envName)
		}
	}
	return targetEnvironments, ignoredForWebhookEnvs, nil
}

func setTargetEnvironmentsForPromote(ctx context.Context, pipelineInfo *model.PipelineInfo) ([]string, error) {
	var errs []error
	if len(pipelineInfo.GetRadixPromoteDeployment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote deployment name"))
	}
	if len(pipelineInfo.GetRadixPromoteFromEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote source environment name"))
	}
	if len(pipelineInfo.GetRadixDeployToEnvironment()) == 0 {
		errs = append(errs, fmt.Errorf("missing promote target environment name"))
	}
	if len(errs) > 0 {
		log.Ctx(ctx).Info().Msg("Pipeline type: promote")
		return nil, errors.Join(errs...)
	}
	log.Ctx(ctx).Info().Msgf("promote the deployment %s from the environment %s to %s", pipelineInfo.GetRadixPromoteDeployment(), pipelineInfo.GetRadixPromoteFromEnvironment(), pipelineInfo.GetRadixDeployToEnvironment())
	return []string{pipelineInfo.GetRadixDeployToEnvironment()}, nil // run Tekton pipelines for the promote target environment
}

func setTargetEnvironmentsForDeploy(ctx context.Context, pipelineInfo *model.PipelineInfo) ([]string, error) {
	targetEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	if len(targetEnvironment) == 0 {
		return nil, fmt.Errorf("no target environment is specified for the deploy pipeline")
	}
	log.Ctx(ctx).Info().Msgf("Target environment: %v", targetEnvironment)
	log.Ctx(ctx).Info().Msgf("Pipeline type: %s", pipelineInfo.GetRadixPipelineType())
	return []string{targetEnvironment}, nil
}
