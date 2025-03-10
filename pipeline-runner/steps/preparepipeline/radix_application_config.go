package preparepipeline

import (
	"context"
	"fmt"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pipeline-runner/utils/configmap"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
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

	ctx.targetEnvironments, err = internal.SetTargetEnvironments(ctx.GetPipelineInfo())
	if err != nil {
		return err
	}

	return ctx.preparePipelinesJob()
}
