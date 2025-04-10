package internal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

// GetPipelineTargetEnvironments Get target environments for the pipeline
func GetPipelineTargetEnvironments(pipelineInfo *model.PipelineInfo) ([]string, error) {
	log.Debug().Msg("Set target environment")
	switch pipelineInfo.GetRadixPipelineType() {
	case v1.ApplyConfig:
		return nil, nil
	case v1.Promote:
		return setTargetEnvironmentsForPromote(pipelineInfo)
	case v1.Deploy:
		return setTargetEnvironmentsForDeploy(pipelineInfo)
	}
	var targetEnvironments []string
	deployToEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	environments := applicationconfig.GetTargetEnvironments(pipelineInfo.GetBranch(), pipelineInfo.GetRadixApplication(), pipelineInfo.PipelineArguments.TriggeredFromWebhook)
	for _, envName := range environments {
		if len(deployToEnvironment) == 0 || deployToEnvironment == envName {
			targetEnvironments = append(targetEnvironments, envName)
		}
	}
	if len(targetEnvironments) > 0 {
		log.Info().Msgf("Environment(s) %v are mapped to the branch %s.", getEnvironmentList(targetEnvironments), pipelineInfo.GetBranch())
	} else {
		log.Info().Msgf("No environments are mapped to the branch %s.", pipelineInfo.GetBranch())
	}
	log.Info().Msgf("Pipeline type: %s", pipelineInfo.GetRadixPipelineType())
	return targetEnvironments, nil
}

func setTargetEnvironmentsForPromote(pipelineInfo *model.PipelineInfo) ([]string, error) {
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
		log.Info().Msg("Pipeline type: promote")
		return nil, errors.Join(errs...)
	}
	log.Info().Msgf("promote the deployment %s from the environment %s to %s", pipelineInfo.GetRadixPromoteDeployment(), pipelineInfo.GetRadixPromoteFromEnvironment(), pipelineInfo.GetRadixDeployToEnvironment())
	return []string{pipelineInfo.GetRadixDeployToEnvironment()}, nil // run Tekton pipelines for the promote target environment
}

func setTargetEnvironmentsForDeploy(pipelineInfo *model.PipelineInfo) ([]string, error) {
	targetEnvironment := pipelineInfo.GetRadixDeployToEnvironment()
	if len(targetEnvironment) == 0 {
		return nil, fmt.Errorf("no target environment is specified for the deploy pipeline")
	}
	log.Info().Msgf("Target environment: %v", targetEnvironment)
	log.Info().Msgf("Pipeline type: %s", pipelineInfo.GetRadixPipelineType())
	return []string{targetEnvironment}, nil
}

func getEnvironmentList(environmentNames []string) string {
	return strings.Join(environmentNames, ", ")
}
