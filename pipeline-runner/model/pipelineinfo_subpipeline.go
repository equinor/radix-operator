package model

import (
	"fmt"

	subpipeline "github.com/equinor/radix-operator/pipeline-runner/internal/subpipeline"
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const (
	subPipelineRadixParamName = "radix"
	subPipelineImageParamName = "radix-image"
)

type SubPipelineParams struct {
	PipelineType string `propname:"pipeline-type"`
	Environment  string `propname:"environment"`
	GitSSHUrl    string `propname:"git-ssh-url"`
	GitRef       string `propname:"git-ref"`
	GitRefType   string `propname:"git-ref-type"`
	GitCommit    string `propname:"git-commit"`
	GitTags      string `propname:"git-tags"`
}

// EnvironmentComponentImages maps environment name to component images
type EnvironmentComponentImages map[string][]string

// GetSubPipelineParamSpecsForEnvironment returns the Tekton ParamSpecs to be appended to a Pipeline's
// parameter declarations for the given environment. These specs define the shape of the reserved
// "radix" and "radix-image" object parameters so that tasks within the Pipeline can reference them.
func (p PipelineInfo) GetSubPipelineParamSpecsForEnvironment(envName string) (pipelinev1.ParamSpecs, error) {
	var specs pipelinev1.ParamSpecs

	paramSpec, err := subpipeline.ObjectParamSpec(subPipelineRadixParamName, p.EnvironmentSubPipelineParams[envName])
	if err != nil {
		return nil, fmt.Errorf("failed to generate radix param for tasks: %w", err)
	}
	specs = append(specs, paramSpec)

	if envImages := p.EnvironmentSubPipelineImageParams[envName]; len(envImages) > 0 {
		spec, err := subpipeline.ObjectParamSpec(subPipelineImageParamName, sliceToMap(envImages))
		if err != nil {
			return nil, fmt.Errorf("failed to generate image param for tasks: %w", err)
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// GetSubPipelineParamValuesForEnvironment returns the Tekton Params with actual values to be set on a
// PipelineRun for the given environment. The "radix" param carries pipeline metadata (git ref, commit,
// etc.) and the "radix-image" param carries a map of component names to their resolved image paths.
func (p PipelineInfo) GetSubPipelineParamValuesForEnvironment(envName string) (pipelinev1.Params, error) {
	var params pipelinev1.Params

	radixParam, err := subpipeline.ObjectParam(subPipelineRadixParamName, p.EnvironmentSubPipelineParams[envName])
	if err != nil {
		return nil, fmt.Errorf("failed to generate radix param values for environment %s: %w", envName, err)
	}
	params = append(params, radixParam)

	if envImages := p.EnvironmentSubPipelineImageParams[envName]; len(envImages) > 0 {
		mergedImages := make(map[string]string, len(envImages))
		for _, image := range envImages {
			mergedImages[image] = ""
		}
		if deployImages, ok := p.DeployEnvironmentComponentImages[envName]; ok {
			for componentName, img := range deployImages {
				if _, exists := mergedImages[componentName]; exists {
					mergedImages[componentName] = img.ImagePath
				}
			}
		}
		imageParam, err := subpipeline.ObjectParam(subPipelineImageParamName, mergedImages)
		if err != nil {
			return nil, fmt.Errorf("failed to generate image param values for environment %s: %w", envName, err)
		}
		params = append(params, imageParam)
	}

	return params, nil
}

// GetSubPipelineParamReferencesForEnvironment returns the Tekton Params whose values are Tekton
// variable-substitution references (e.g. "$(params.radix.git-ref)"). These are intended to be
// appended to a Pipeline task's parameter list so that each task receives the reserved params
// forwarded from the Pipeline-level object parameters.
func (p PipelineInfo) GetSubPipelineParamReferencesForEnvironment(envName string) (pipelinev1.Params, error) {
	paramSpecs, err := p.GetSubPipelineParamSpecsForEnvironment(envName)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-pipeline param specs for environment %s: %w", envName, err)
	}
	paramSpecsByName := make(map[string]pipelinev1.ParamSpec, len(paramSpecs))
	for _, ps := range paramSpecs {
		paramSpecsByName[ps.Name] = ps
	}

	var refs pipelinev1.Params

	if radixParamSpec, ok := paramSpecsByName[subPipelineRadixParamName]; ok {
		paramRef, err := subpipeline.ObjectParamReference(subPipelineRadixParamName, p.EnvironmentSubPipelineParams[envName], radixParamSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to generate radix param reference for environment %s: %w", envName, err)
		}
		refs = append(refs, paramRef)
	}

	if imageParamSpec, ok := paramSpecsByName[subPipelineImageParamName]; ok {
		paramRef, err := subpipeline.ObjectParamReference(subPipelineImageParamName, sliceToMap(p.EnvironmentSubPipelineImageParams[envName]), imageParamSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to generate image param reference for environment %s: %w", envName, err)
		}
		refs = append(refs, paramRef)
	}

	return refs, nil
}

func sliceToMap(slice []string) map[string]string {
	result := make(map[string]string, len(slice))
	for _, item := range slice {
		result[item] = ""
	}
	return result
}
