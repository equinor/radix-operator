package internal

import (
	"context"
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/pipeline/application"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
)

// LoadRadixAppConfig Load Radix config file and create RadixApplication
func LoadRadixAppConfig(radixClient radixclient.Interface, pipelineInfo *model.PipelineInfo) (*radixv1.RadixApplication, error) {
	radixConfigFileFullName := pipelineInfo.GetRadixConfigFileInWorkspace()
	configFileContent, err := os.ReadFile(radixConfigFileFullName)
	if err != nil {
		return nil, fmt.Errorf("error reading the Radix config file %s: %v", radixConfigFileFullName, err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", radixConfigFileFullName)

	radixApplication, err := application.CreateRadixApplication(context.Background(), radixClient, pipelineInfo.GetAppName(), pipelineInfo.GetDNSConfig(), string(configFileContent))
	if err != nil {
		return nil, err
	}
	log.Debug().Msg("Radix Application has been loaded")
	return radixApplication, err
}
