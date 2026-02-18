package internal

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
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type envInfo struct {
	envName  string
	activeRd radixv1.RadixDeployment
}

type envInfoList []envInfo

type deployHandler struct {
	pipelineInfo     *model.PipelineInfo
	radixClient      versioned.Interface
	kubeClient       kubernetes.Interface
	rdWatcher        watcher.RadixDeploymentWatcher
	featureProviders []FeatureProvider
}

func NewHandler(pipelineInfo *model.PipelineInfo, radixClient versioned.Interface, kubeClient kubernetes.Interface, rdWatcher watcher.RadixDeploymentWatcher, featureProviders []FeatureProvider) *deployHandler {
	return &deployHandler{
		pipelineInfo:     pipelineInfo,
		radixClient:      radixClient,
		kubeClient:       kubeClient,
		rdWatcher:        rdWatcher,
		featureProviders: featureProviders,
	}
}

func (h *deployHandler) Deploy(ctx context.Context) error {
	if len(h.featureProviders) == 0 {
		return nil
	}

	candidates, err := h.getEnvironmentCandidates(ctx)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	deployments, err := h.buildDeploymentsForEnvironments(ctx, candidates)
	if err != nil {
		return fmt.Errorf("failed to build Radix deployments: %w", err)
	}

	if err := h.applyDeployments(ctx, deployments); err != nil {
		return fmt.Errorf("failed to apply Radix deployments: %w", err)
	}

	return nil
}

func (h *deployHandler) getEnvironmentCandidates(ctx context.Context) (envInfoList, error) {
	allEnvs := envInfoList{}
	for _, env := range h.pipelineInfo.RadixApplication.Spec.Environments {
		envNs := utils.GetEnvironmentNamespace(h.pipelineInfo.RadixApplication.GetName(), env.Name)
		rd, err := internal.GetActiveRadixDeployment(ctx, h.radixClient, h.kubeClient, envNs)
		if err != nil {
			return nil, fmt.Errorf("failed to get active Radix deployment for environment %s: %w", env.Name, err)
		}
		if rd != nil {
			allEnvs = append(allEnvs, envInfo{envName: env.Name, activeRd: *rd})
		} else {
			log.Ctx(ctx).Info().Msgf("Ignoring environment %s because does not have an active Radix deploymewnt", env.Name)
		}
	}

	deployEnvs := envInfoList{}
	for _, envInfo := range allEnvs {
		if slices.ContainsFunc(h.featureProviders, func(deployer FeatureProvider) bool {
			return deployer.IsEnabledForEnvironment(envInfo.envName, *h.pipelineInfo.RadixApplication, envInfo.activeRd)
		}) {
			deployEnvs = append(deployEnvs, envInfo)
		}
	}

	return deployEnvs, nil
}

func (h *deployHandler) applyDeployments(ctx context.Context, rdList []*radixv1.RadixDeployment) error {
	if len(rdList) == 0 {
		return nil
	}

	for _, rd := range rdList {
		log.Ctx(ctx).Info().Msgf("Apply Radix deployment %s to environment %s", rd.GetName(), rd.Spec.Environment)
		_, err := h.radixClient.RadixV1().RadixDeployments(rd.GetNamespace()).Create(ctx, rd, metav1.CreateOptions{})
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

func (h *deployHandler) buildDeploymentsForEnvironments(ctx context.Context, environments envInfoList) ([]*radixv1.RadixDeployment, error) {
	var rdList []*radixv1.RadixDeployment
	for _, envInfo := range environments {
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
	sourceRd, err := internal.ConstructForTargetEnvironment(ctx,
		h.pipelineInfo,
		model.TargetEnvironment{Environment: envInfo.envName, ActiveRadixDeployment: &envInfo.activeRd},
		envInfo.activeRd.Annotations[kube.RadixCommitAnnotation],
		envInfo.activeRd.Annotations[kube.RadixGitTagsAnnotation],
		envInfo.activeRd.Annotations[kube.RadixConfigHash],
		envInfo.activeRd.Annotations[kube.RadixBuildSecretHash])
	if err != nil {
		return nil, fmt.Errorf("failed to construct Radix deployment: %w", err)
	}

	targetRd := envInfo.activeRd.DeepCopy()
	targetRd.ObjectMeta = *sourceRd.ObjectMeta.DeepCopy()
	targetRd.Status = radixv1.RadixDeployStatus{}

	for _, updater := range h.featureProviders {
		if err := updater.Mutate(*targetRd, *sourceRd); err != nil {
			return nil, fmt.Errorf("failed to apply configuration to Radix deployment: %w", err)
		}
	}

	return targetRd, nil
}
