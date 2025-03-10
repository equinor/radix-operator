package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-operator/pipeline-runner/utils/configmap"
	"github.com/equinor/radix-operator/pkg/apis/applicationconfig"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

// ProcessRadixAppConfig Load Radix config file to a ConfigMap and create RadixApplication
func (ctx *pipelineContext) ProcessRadixAppConfig() error {
	configFileContent, err := configmap.ReadFromRadixConfigFile(ctx.GetPipelineInfo().GetRadixConfigFile())
	if err != nil {
		return fmt.Errorf("error reading the Radix config file %s: %v", ctx.GetPipelineInfo().GetRadixConfigFile(), err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", ctx.GetPipelineInfo().GetRadixConfigFile())

	radixApplication, err := application.CreateRadixApplication(context.Background(), ctx.radixClient, ctx.GetPipelineInfo().GetAppName(), ctx.GetPipelineInfo().GetDNSConfig(), configFileContent)
	if err != nil {
		return err
	}
	ctx.pipelineInfo.SetRadixApplication(radixApplication)
	log.Debug().Msg("Radix Application has been loaded")

	if err = ctx.setTargetEnvironments(); err != nil {
		return err
	}

	return ctx.preparePipelinesJob()
}

func (ctx *pipelineContext) setTargetEnvironments() error {
	log.Debug().Msg("Set target environment")
	switch ctx.GetPipelineInfo().GetRadixPipelineType() {
	case radixv1.ApplyConfig:
		return nil
	case radixv1.Promote:
		return ctx.setTargetEnvironmentsForPromote()
	case radixv1.Deploy:
		return ctx.setTargetEnvironmentsForDeploy()
	}
	targetEnvironments := applicationconfig.GetTargetEnvironments(ctx.pipelineInfo.GetBranch(), ctx.GetPipelineInfo().GetRadixApplication())
	ctx.targetEnvironments = []string{}
	deployToEnvironment := ctx.GetPipelineInfo().GetRadixDeployToEnvironment()
	for _, envName := range targetEnvironments {
		if len(deployToEnvironment) == 0 || deployToEnvironment == envName {
			ctx.targetEnvironments = append(ctx.targetEnvironments, envName)
		}
	}
	if len(ctx.targetEnvironments) > 0 {
		log.Info().Msgf("Environment(s) %v are mapped to the branch %s.", getEnvironmentList(ctx.targetEnvironments), ctx.GetPipelineInfo().GetBranch())
	} else {
		log.Info().Msgf("No environments are mapped to the branch %s.", ctx.GetPipelineInfo().GetBranch())
	}
	log.Info().Msgf("Pipeline type: %s", ctx.GetPipelineInfo().GetRadixPipelineType())
	return nil
}

func (ctx *pipelineContext) setTargetEnvironmentsForPromote() error {
	var errs []error
	pipelineInfo := ctx.GetPipelineInfo()
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
		return errors.Join(errs...)
	}
	ctx.targetEnvironments = []string{pipelineInfo.GetRadixDeployToEnvironment()} // run Tekton pipelines for the promote target environment
	log.Info().Msgf("promote the deployment %s from the environment %s to %s", pipelineInfo.GetRadixPromoteDeployment(), pipelineInfo.GetRadixPromoteFromEnvironment(), pipelineInfo.GetRadixDeployToEnvironment())
	return nil
}

func (ctx *pipelineContext) setTargetEnvironmentsForDeploy() error {
	targetEnvironment := ctx.GetPipelineInfo().GetRadixDeployToEnvironment()
	if len(targetEnvironment) == 0 {
		return fmt.Errorf("no target environment is specified for the deploy pipeline")
	}
	ctx.targetEnvironments = []string{targetEnvironment}
	log.Info().Msgf("Target environment: %v", targetEnvironment)
	log.Info().Msgf("Pipeline type: %s", ctx.GetPipelineInfo().GetRadixPipelineType())
	return nil
}

func getEnvironmentList(environmentNames []string) string {
	return strings.Join(environmentNames, ", ")
}
