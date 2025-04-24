package promote

import (
	"context"
	"fmt"

	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/pipeline"
	"github.com/equinor/radix-operator/pkg/apis/radixvalidators"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PromoteStepImplementation Step to promote deployment to another environment,
// or inside environment
type PromoteStepImplementation struct {
	stepType pipeline.StepType
	model.DefaultStepImplementation
}

// NewPromoteStep Constructor
func NewPromoteStep() model.Step {
	return &PromoteStepImplementation{
		stepType: pipeline.PromoteStep,
	}
}

// ImplementationForType Override of default step method
func (cli *PromoteStepImplementation) ImplementationForType() pipeline.StepType {
	return cli.stepType
}

// SucceededMsg Override of default step method
func (cli *PromoteStepImplementation) SucceededMsg() string {
	return fmt.Sprintf("Successful promotion for application %s", cli.GetAppName())
}

// ErrorMsg Override of default step method
func (cli *PromoteStepImplementation) ErrorMsg(err error) string {
	return fmt.Sprintf("Promotion failed for application %s. Error: %v", cli.GetAppName(), err)
}

// Run Override of default step method
func (cli *PromoteStepImplementation) Run(ctx context.Context, pipelineInfo *model.PipelineInfo) error {
	log.Ctx(ctx).Info().Msgf("Promoting %s for application %s from %s to %s", pipelineInfo.PipelineArguments.DeploymentName, cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment, pipelineInfo.PipelineArguments.ToEnvironment)
	if err := areArgumentsValid(pipelineInfo.PipelineArguments); err != nil {
		return err
	}

	fromNs, toNs, err := cli.getNamespaces(ctx, pipelineInfo)
	if err != nil {
		return err
	}

	rd, err := cli.GetRadixClient().RadixV1().RadixDeployments(fromNs).Get(ctx, pipelineInfo.PipelineArguments.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return NonExistingDeployment(pipelineInfo.PipelineArguments.DeploymentName)
	}

	radixDeployment := rd.DeepCopy()
	radixDeployment.Name = utils.GetDeploymentName(pipelineInfo.PipelineArguments.ToEnvironment, pipelineInfo.PipelineArguments.ImageTag)

	activeRadixDeployment, err := cli.GetKubeUtil().GetActiveDeployment(ctx, toNs)
	if err != nil {
		return err
	}

	if radixDeployment.GetAnnotations() == nil {
		radixDeployment.ObjectMeta.Annotations = make(map[string]string)
	}
	if _, isRestored := radixDeployment.Annotations[kube.RestoredStatusAnnotation]; isRestored {
		// RA-817: Promotion reuses annotation - RD get inactive status
		radixDeployment.Annotations[kube.RestoredStatusAnnotation] = ""
	}
	radixDeployment.Annotations[kube.RadixDeploymentPromotedFromDeploymentAnnotation] = rd.GetName()
	radixDeployment.Annotations[kube.RadixDeploymentPromotedFromEnvironmentAnnotation] = pipelineInfo.PipelineArguments.FromEnvironment

	radixDeployment.ResourceVersion = ""
	radixDeployment.Namespace = toNs
	radixDeployment.Labels[kube.RadixEnvLabel] = pipelineInfo.PipelineArguments.ToEnvironment
	radixDeployment.Labels[kube.RadixJobNameLabel] = pipelineInfo.PipelineArguments.JobName
	radixDeployment.Spec.Environment = pipelineInfo.PipelineArguments.ToEnvironment

	if err = internal.MergeRadixDeploymentWithRadixApplicationAttributes(pipelineInfo.RadixApplication, activeRadixDeployment, radixDeployment, pipelineInfo.PipelineArguments.ToEnvironment, pipelineInfo.DeployEnvironmentComponentImages[pipelineInfo.PipelineArguments.ToEnvironment]); err != nil {
		return err
	}

	if err = radixvalidators.CanRadixDeploymentBeInserted(radixDeployment); err != nil {
		return err
	}

	_, err = cli.GetRadixClient().RadixV1().RadixDeployments(toNs).Create(ctx, radixDeployment, metav1.CreateOptions{})
	return err
}

func (cli *PromoteStepImplementation) getNamespaces(ctx context.Context, pipelineInfo *model.PipelineInfo) (string, string, error) {
	fromNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.FromEnvironment)
	if _, err := cli.GetKubeClient().CoreV1().Namespaces().Get(ctx, fromNs, metav1.GetOptions{}); err != nil {
		return "", "", NonExistingFromEnvironment(pipelineInfo.PipelineArguments.FromEnvironment)
	}
	toNs := utils.GetEnvironmentNamespace(cli.GetAppName(), pipelineInfo.PipelineArguments.ToEnvironment)
	if _, err = cli.GetKubeClient().CoreV1().Namespaces().Get(ctx, toNs, metav1.GetOptions{}); err != nil {
		return "", "", NonExistingToEnvironment(pipelineInfo.PipelineArguments.ToEnvironment)
	}
	return fromNs, toNs, nil
}

func areArgumentsValid(arguments model.PipelineArguments) error {
	if arguments.FromEnvironment == "" {
		return EmptyArgument("From environment")
	}

	if arguments.ToEnvironment == "" {
		return EmptyArgument("To environment")
	}

	if arguments.DeploymentName == "" {
		return EmptyArgument("Deployment name")
	}

	if arguments.JobName == "" {
		return EmptyArgument("Job name")
	}

	if arguments.ImageTag == "" {
		return EmptyArgument("Image tag")
	}

	return nil
}
