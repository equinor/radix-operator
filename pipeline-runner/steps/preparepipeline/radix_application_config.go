package preparepipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
)

// LoadRadixAppConfig Load Radix config file and create RadixApplication to the PipelineInfo.RadixApplication. Output is in the PipelineInfo.BuildContext
func (pipelineCtx *pipelineContext) LoadRadixAppConfig() (*radixv1.RadixApplication, error) {
	radixConfigFileFullName := pipelineCtx.GetPipelineInfo().GetRadixConfigFileInWorkspace()
	configFileContent, err := os.ReadFile(radixConfigFileFullName)
	if err != nil {
		return nil, fmt.Errorf("error reading the Radix config file %s: %v", radixConfigFileFullName, err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", radixConfigFileFullName)

	radixApplication, err := application.CreateRadixApplication(context.Background(), pipelineCtx.GetRadixClient(), pipelineCtx.GetPipelineInfo().GetAppName(), pipelineCtx.GetPipelineInfo().GetDNSConfig(), string(configFileContent))
	if err != nil {
		return nil, err
	}
	log.Debug().Msg("Radix Application has been loaded")
	return radixApplication, err
}
