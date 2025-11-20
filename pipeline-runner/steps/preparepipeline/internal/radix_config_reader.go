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

// RadixConfigReader interface for reading Radix config file
type RadixConfigReader interface {
	Read(pipelineInfo *model.PipelineInfo) (*radixv1.RadixApplication, error)
}

type radixConfigReader struct {
	radixClient radixclient.Interface
}

// NewRadixConfigReader creates a new instance of RadixConfigReader
func NewRadixConfigReader(radixClient radixclient.Interface) RadixConfigReader {
	return &radixConfigReader{
		radixClient: radixClient,
	}
}

// Read Reads Radix config file and create RadixApplication
func (r *radixConfigReader) Read(pipelineInfo *model.PipelineInfo) (*radixv1.RadixApplication, error) {
	radixConfigFileFullName := pipelineInfo.GetRadixConfigFileInWorkspace()
	configFileContent, err := os.ReadFile(radixConfigFileFullName)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", radixConfigFileFullName, err)
	}
	log.Debug().Msgf("Radix config file %s has been loaded", radixConfigFileFullName)

	radixApplication, err := application.ParseRadixApplication(context.Background(), r.radixClient, pipelineInfo.GetAppName(), configFileContent)
	if err != nil {
		return nil, err
	}
	log.Debug().Msg("Radix Application has been loaded")
	return radixApplication, err
}
