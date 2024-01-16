package steps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployStepImplementation Step to deploy RD into environment
type DeployStepImplementation struct {
	stepType         pipeline.StepType
	namespaceWatcher kube.NamespaceWatcher
	model.DefaultStepImplementation
}

// NewDeployStep Constructor
func NewDeployStep(namespaceWatcher kube.NamespaceWatcher) model.Step {
	return &DeployStepImplementation{
		stepType:         pipeline.DeployStep,
		namespaceWatcher: namespaceWatcher,
	}
}

// ImplementationForType Override of default step method
func (cli *DeployStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *DeployStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Succeded: deploy application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *DeployStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Failed to deploy application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *DeployStepImplementation) Run(pipelineInfo *model.PipelineInfo) error {
	err := cli.deploy(pipelineInfo)
	return err
}

// Deploy Handles deploy step of the pipeline
func (cli *DeployStepImplementation) deploy(pipelineInfo *model.PipelineInfo) error {
	appName := cli.GetAppName()
	log.Infof("Deploying app %s", appName)

	if len(pipelineInfo.TargetEnvironments) == 0 {
		log.Infof("skip deploy step as branch %s is not mapped to any environment", pipelineInfo.PipelineArguments.Branch)
		return nil
	}

	for _, env := range pipelineInfo.TargetEnvironments {
		err := cli.deployToEnv(appName, env, pipelineInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cli *DeployStepImplementation) deployToEnv(appName, env string, pipelineInfo *model.PipelineInfo) error {
	err := cli.validate(pipelineInfo, env)
	if err != nil {
		return err
	}
	defaultEnvVars, err := getDefaultEnvVars(pipelineInfo)
	if err != nil {
		return fmt.Errorf("failed to retrieve default env vars for RadixDeployment in app  %s. %v", appName, err)
	}

	if commitID, ok := defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable]; !ok || len(commitID) == 0 {
		defaultEnvVars[defaults.RadixCommitHashEnvironmentVariable] = pipelineInfo.PipelineArguments.CommitID // Commit ID specified by job arguments
	}

	radixApplicationHash, err := createRadixApplicationHash(pipelineInfo.RadixApplication)
	if err != nil {
		return err
	}

	buildSecretHash, err := createBuildSecretHash(pipelineInfo.BuildSecret)
	if err != nil {
		return err
	}

	preservingDeployComponents, preservingDeployJobComponents, err := cli.getDeployComponents(pipelineInfo.RadixApplication, env, pipelineInfo.PipelineArguments.Components)
	if err != nil {
		return err
	}
	radixDeployment, err := deployment.ConstructForTargetEnvironment(
		pipelineInfo.RadixApplication,
		pipelineInfo.PipelineArguments.JobName,
		pipelineInfo.PipelineArguments.ImageTag,
		pipelineInfo.PipelineArguments.Branch,
		pipelineInfo.DeployEnvironmentComponentImages[env],
		env,
		defaultEnvVars,
		radixApplicationHash,
		buildSecretHash,
		preservingDeployComponents,
		preservingDeployJobComponents,
	)

	if err != nil {
		return fmt.Errorf("failed to create radix deployments objects for app %s. %v", appName, err)
	}

	err = cli.namespaceWatcher.WaitFor(utils.GetEnvironmentNamespace(cli.GetAppName(), env))
	if err != nil {
		return fmt.Errorf("failed to get environment namespace, %s, for app %s. %v", env, appName, err)
	}

	log.Infof("Apply radix deployment %s on env %s", radixDeployment.GetName(), radixDeployment.GetNamespace())
	_, err = cli.GetRadixclient().RadixV1().RadixDeployments(radixDeployment.GetNamespace()).Create(context.TODO(), radixDeployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to apply radix deployment for app %s to environment %s. %v", appName, env, err)
	}

	return nil
}

func (cli *DeployStepImplementation) validate(pipelineInfo *model.PipelineInfo, env string) error {
	if len(pipelineInfo.PipelineArguments.Components) == 0 {
		return nil
	}
	var errs []error
	componentsMap := getComponentMap(pipelineInfo)
	for _, componentName := range pipelineInfo.PipelineArguments.Components {
		component, ok := componentsMap[componentName]
		if !ok {
			errs = append(errs, fmt.Errorf("requested component %s does not exist", componentName))
		} else if !component.GetEnabledForEnvironment(env) {
			errs = append(errs, fmt.Errorf("requested component %s is disabled in the environment %s", componentName, env))
		}
	}
	return errors.Join(errs...)
}

func (cli *DeployStepImplementation) getDeployComponents(radixApplication *radixv1.RadixApplication, env string, components []string) ([]radixv1.RadixDeployComponent, []radixv1.RadixDeployJobComponent, error) {
	if len(components) == 0 {
		return nil, nil, nil
	}
	log.Infof("Deploy only following component(s): %s", strings.Join(components, ","))
	componentNames := slice.Reduce(components, make(map[string]bool), func(acc map[string]bool, name string) map[string]bool {
		acc[name] = true
		return acc
	})
	activeRadixDeployment, err := cli.GetKubeutil().GetActiveDeployment(utils.GetEnvironmentNamespace(radixApplication.GetName(), env))
	if err != nil {
		return nil, nil, err
	}
	if activeRadixDeployment == nil {
		return nil, nil, nil
	}
	deployComponents := slice.FindAll(activeRadixDeployment.Spec.Components, func(component radixv1.RadixDeployComponent) bool {
		return !componentNames[component.GetName()]
	})
	deployJobComponents := slice.FindAll(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
		return !componentNames[component.GetName()]
	})
	return deployComponents, deployJobComponents, nil
}

func getComponentMap(pipelineInfo *model.PipelineInfo) map[string]radixv1.RadixCommonComponent {
	componentsMap := slice.Reduce(pipelineInfo.RadixApplication.Spec.Components, make(map[string]radixv1.RadixCommonComponent), func(acc map[string]radixv1.RadixCommonComponent, component radixv1.RadixComponent) map[string]radixv1.RadixCommonComponent {
		acc[component.GetName()] = &component
		return acc
	})
	componentsMap = slice.Reduce(pipelineInfo.RadixApplication.Spec.Jobs, componentsMap, func(acc map[string]radixv1.RadixCommonComponent, jobComponent radixv1.RadixJobComponent) map[string]radixv1.RadixCommonComponent {
		acc[jobComponent.GetName()] = &jobComponent
		return acc
	})
	return componentsMap
}

func getDefaultEnvVars(pipelineInfo *model.PipelineInfo) (radixv1.EnvVarsMap, error) {
	gitCommitHash := pipelineInfo.GitCommitHash
	gitTags := pipelineInfo.GitTags

	envVarsMap := make(radixv1.EnvVarsMap)
	envVarsMap[defaults.RadixCommitHashEnvironmentVariable] = gitCommitHash
	envVarsMap[defaults.RadixGitTagsEnvironmentVariable] = gitTags

	return envVarsMap, nil
}
