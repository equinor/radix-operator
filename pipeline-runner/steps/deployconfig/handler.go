package deployconfig

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-operator/pipeline-runner/internal/watcher"
	"github.com/equinor/radix-operator/pipeline-runner/model"
	"github.com/equinor/radix-operator/pipeline-runner/steps/internal"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type envInfo struct {
	envName  string
	activeRd *radixv1.RadixDeployment
}

type envInfoList []envInfo

type deployHandler struct {
	updaters     []DeploymentUpdater
	kubeutil     *kube.Kube
	pipelineInfo model.PipelineInfo
	rdWatcher    watcher.RadixDeploymentWatcher
}

func (h *deployHandler) deploy(ctx context.Context) error {
	envsToDeploy, err := h.getEnvironmentsToDeploy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get list of environments to deploy: %w", err)
	}

	if err := h.validateEnvironmentsToDeploy(envsToDeploy); err != nil {
		return fmt.Errorf("failed to validate environments: %w", err)
	}

	return h.deployEnvironments(ctx, envsToDeploy)
}

func (h *deployHandler) validateEnvironmentsToDeploy(envsToDeploy envInfoList) error {
	for _, envInfo := range envsToDeploy {
		if envInfo.activeRd == nil {
			return fmt.Errorf("cannot deploy to environment %s because it does not have an active Radix deployment", envInfo.envName)
		}
	}
	return nil
}

func (h *deployHandler) getEnvironmentsToDeploy(ctx context.Context) (envInfoList, error) {
	allEnvs := envInfoList{}
	for _, env := range h.pipelineInfo.RadixApplication.Spec.Environments {
		envNs := utils.GetEnvironmentNamespace(h.pipelineInfo.RadixApplication.GetName(), env.Name)
		rd, err := internal.GetActiveRadixDeployment(ctx, h.kubeutil, envNs)
		if err != nil {
			return nil, fmt.Errorf("failed to get active Radix deployment for environment %s: %w", env.Name, err)
		}
		allEnvs = append(allEnvs, envInfo{envName: env.Name, activeRd: rd})
	}

	deployEnvs := envInfoList{}
	for _, envInfo := range allEnvs {
		if slices.ContainsFunc(h.updaters, func(deployer DeploymentUpdater) bool {
			return deployer.MustDeployEnvironment(envInfo.envName, h.pipelineInfo.RadixApplication, envInfo.activeRd)
		}) {
			deployEnvs = append(deployEnvs, envInfo)
		}
	}

	return deployEnvs, nil
}

func (h *deployHandler) deployEnvironments(ctx context.Context, envs envInfoList) error {
	rdList, err := h.buildDeployments(ctx, envs)
	if err != nil {
		return fmt.Errorf("failed to build Radix deployments: %w", err)
	}

	if err := h.applyDeployments(ctx, rdList); err != nil {
		return fmt.Errorf("failed to apply Radix deployments: %w", err)
	}

	return nil
}

func (h *deployHandler) applyDeployments(ctx context.Context, rdList []*radixv1.RadixDeployment) error {
	if len(rdList) == 0 {
		return nil
	}

	for _, rd := range rdList {
		log.Ctx(ctx).Info().Msgf("Apply Radix deployment %s to environment %s", rd.GetName(), rd.Spec.Environment)
		_, err := h.kubeutil.RadixClient().RadixV1().RadixDeployments(rd.GetNamespace()).Create(ctx, rd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to apply Radix deployment %s to environment %s: %w", rd.GetName(), rd.Spec.Environment, err)
		}
	}

	var g errgroup.Group
	for _, rd := range rdList {
		g.Go(func() error {
			if err := h.rdWatcher.WaitForActive(ctx, rd.GetNamespace(), rd.GetName()); err != nil {
				return fmt.Errorf("failed to wait for activation of Radix deployment %s in environment %s: %w", rd.GetName(), rd.Spec.Environment, err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to wait for activation of Radix deployments: %w", err)
	}

	return nil
}

func (h *deployHandler) buildDeployments(ctx context.Context, envs envInfoList) ([]*radixv1.RadixDeployment, error) {
	var rdList []*radixv1.RadixDeployment

	for _, envInfo := range envs {
		rd, err := h.buildDeployment(ctx, envInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to build Radix deployment for environment %s: %w", envInfo.envName, err)
		}

		// Add deployment if new deployment spec differs from active deployment
		if !cmp.Equal(rd.Spec, envInfo.activeRd.Spec, cmpopts.EquateEmpty()) {
			rdList = append(rdList, rd)
		}
	}

	return rdList, nil
}

func (h *deployHandler) buildDeployment(ctx context.Context, envInfo envInfo) (*radixv1.RadixDeployment, error) {
	// TODO: should we use the new RA hash or hash from current active RD?
	// Both can cause inconsitency issues since we technically do not apply everything from RA to the RDs
	// radixApplicationHash, err := internal.CreateRadixApplicationHash(h.pipelineInfo.RadixApplication)
	// if err != nil {
	// 	return nil, err
	// }

	sourceRd, err := internal.ConstructForTargetEnvironment(
		ctx,
		h.pipelineInfo.RadixApplication,
		envInfo.activeRd,
		h.pipelineInfo.PipelineArguments.JobName,
		h.pipelineInfo.PipelineArguments.ImageTag,
		envInfo.activeRd.Annotations[kube.RadixBranchAnnotation],
		envInfo.activeRd.Annotations[kube.RadixCommitAnnotation],
		envInfo.activeRd.Annotations[kube.RadixGitTagsAnnotation],
		nil,
		envInfo.envName,
		envInfo.activeRd.Annotations[kube.RadixConfigHash],
		envInfo.activeRd.Annotations[kube.RadixBuildSecretHash],
		h.pipelineInfo.PrepareBuildContext,
		h.pipelineInfo.PipelineArguments.ComponentsToDeploy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct Radix deployment: %w", err)
	}

	// TODO: Verify that the same components and jobs exist in both RDs?

	targetRd := envInfo.activeRd.DeepCopy()
	targetRd.ObjectMeta = *sourceRd.ObjectMeta.DeepCopy()

	for _, updater := range h.updaters {
		if err := updater.UpdateDeployment(targetRd, sourceRd); err != nil {
			return nil, fmt.Errorf("failed to apply configu to Radix deployment: %w", err)
		}
	}

	return targetRd, nil
}
