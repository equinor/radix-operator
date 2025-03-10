package preparepipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

// LoadRadixAppConfig Load Radix config file and create RadixApplication to the PipelineInfo.RadixApplication. Output is in the PipelineInfo.BuildContext
func (ctx *pipelineContext) LoadRadixAppConfig() (*radixv1.RadixApplication, error) {
	radixConfigFile := ctx.GetPipelineInfo().GetRadixConfigFile()
	configFileContent, err := os.ReadFile(radixConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading the Radix config file %s: %v", radixConfigFile, err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", radixConfigFile)

	radixApplication, err := application.CreateRadixApplication(context.Background(), ctx.radixClient, ctx.GetPipelineInfo().GetAppName(), ctx.GetPipelineInfo().GetDNSConfig(), string(configFileContent))
	if err != nil {
		return nil, err
	}
	log.Debug().Msg("Radix Application has been loaded")

	// TODO - implicit assignment - use targetEnvironments close where it is needed
	ctx.targetEnvironments, err = internal.SetTargetEnvironments(ctx.GetPipelineInfo())

	return radixApplication, err
}
