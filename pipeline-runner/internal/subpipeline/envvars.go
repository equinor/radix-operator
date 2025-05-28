package subpipeline

import (
	"maps"

	"github.com/equinor/radix-operator/pipeline-runner/model/defaults"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetEnvVars returns environment variables to be used as params for sub-pipelines
func GetEnvVars(ra *radixv1.RadixApplication, envName string) radixv1.EnvVarsMap {
	vars := make(radixv1.EnvVarsMap)
	maps.Insert(vars, maps.All(getPipelineRunParamsFromBuild(ra)))
	maps.Insert(vars, maps.All(getPipelineRunParamsFromEnvironmentBuilds(ra, envName)))

	// Delete AZURE_CLIENT_ID var if explicitly set to empty string in env spec
	if envSpec, ok := ra.GetEnvironmentByName(envName); ok {
		if envSpec.SubPipeline != nil && envSpec.SubPipeline.Identity != nil && envSpec.SubPipeline.Identity.Azure != nil && len(envSpec.SubPipeline.Identity.Azure.ClientId) == 0 {
			delete(vars, defaults.AzureClientIdEnvironmentVariable)
		}
	}

	return vars
}

func getPipelineRunParamsFromBuild(ra *radixv1.RadixApplication) radixv1.EnvVarsMap {
	if ra.Spec.Build == nil {
		return nil
	}

	vars := make(radixv1.EnvVarsMap)
	maps.Insert(vars, maps.All(getBuildIdentity(ra.Spec.Build.SubPipeline)))
	maps.Insert(vars, maps.All(getBuildVariables(ra.Spec.Build.SubPipeline, ra.Spec.Build.Variables)))
	return vars
}

func getPipelineRunParamsFromEnvironmentBuilds(ra *radixv1.RadixApplication, envName string) radixv1.EnvVarsMap {
	vars := make(radixv1.EnvVarsMap)

	if buildEnv, ok := ra.GetEnvironmentByName(envName); ok {
		maps.Insert(vars, maps.All(getBuildIdentity(buildEnv.SubPipeline)))
		maps.Insert(vars, maps.All(getBuildVariables(buildEnv.SubPipeline, buildEnv.Build.Variables)))
	}

	return vars
}

func getBuildVariables(subPipeline *radixv1.SubPipeline, variables radixv1.EnvVarsMap) radixv1.EnvVarsMap {
	if subPipeline != nil {
		return subPipeline.Variables // sub-pipeline variables have higher priority over build variables
	}
	return variables // keep for backward compatibility
}

func getBuildIdentity(subPipeline *radixv1.SubPipeline) radixv1.EnvVarsMap {
	vars := make(radixv1.EnvVarsMap)
	if subPipeline != nil {
		maps.Insert(vars, maps.All(getIdentityToEnvVarsMap(subPipeline.Identity)))
	}
	return vars
}

func getIdentityToEnvVarsMap(identity *radixv1.Identity) radixv1.EnvVarsMap {
	if identity == nil || identity.Azure == nil || len(identity.Azure.ClientId) == 0 {
		return nil
	}

	return radixv1.EnvVarsMap{
		defaults.AzureClientIdEnvironmentVariable: identity.Azure.ClientId,
	}
}
