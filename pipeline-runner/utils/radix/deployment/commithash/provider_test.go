package commithash

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/maps"
	"github.com/equinor/radix-operator/pipeline-runner/utils/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type scenarioGetLastSuccessfulEnvironmentDeployCommits struct {
	name                       string
	environments               []string
	envRadixDeploymentBuilders map[string][]utils.DeploymentBuilder
	expectedEnvCommits         map[string]string
	pipelineJobs               []*v1.RadixJob
}

func TestGetLastSuccessfulEnvironmentDeployCommits(t *testing.T) {
	const appName = "anApp"
	timeNow := time.Now()
	scenarios := []scenarioGetLastSuccessfulEnvironmentDeployCommits{
		{
			name:         "no deployments - no commits",
			environments: []string{"dev", "qa"},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": nil,
			},
			expectedEnvCommits: map[string]string{"dev": "", "qa": ""},
		},
		{
			name:         "deployment has no radix-job - no commit",
			environments: []string{"dev", "qa"},
			pipelineJobs: nil,
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1"),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "", "qa": ""},
		},
		{
			name:         "one deployment with commit in annotation - its commit",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1"),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit1", "qa": ""},
		},
		{
			name:         "one deployment with commit in label - its commit",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithLabel(kube.RadixCommitLabel, "commit1").
						WithLabel(kube.RadixJobNameLabel, "job1"),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit1", "qa": ""},
		},
		{
			name:         "one deployment with commit both in label and annotation - takes commit from annotation",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit-in-annotation",
						}).
						WithLabel(kube.RadixCommitLabel, "commit-in-label").
						WithLabel(kube.RadixJobNameLabel, "job1"),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit-in-annotation", "qa": ""},
		},
		{
			name:         "multiple deployments - gets the commit from the latest deployment",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
				createPipelineJobs(appName, "job2", v1.BuildDeploy),
				createPipelineJobs(appName, "job3", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(1))).
						WithCondition(v1.DeploymentInactive),
					getRadixDeploymentBuilderFor("dev", "rd3").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit3",
						}).
						WithLabel(kube.RadixJobNameLabel, "job3").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(9))).
						WithCondition(v1.DeploymentActive),
					getRadixDeploymentBuilderFor("dev", "rd2").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit2",
						}).
						WithLabel(kube.RadixJobNameLabel, "job2").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(5))).
						WithCondition(v1.DeploymentInactive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit3", "qa": ""},
		},
		{
			name:         "multiple deployments - not affected by status active",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
				createPipelineJobs(appName, "job2", v1.BuildDeploy),
				createPipelineJobs(appName, "job3", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(1))).
						WithCondition(v1.DeploymentInactive),
					getRadixDeploymentBuilderFor("dev", "rd3").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit3",
						}).
						WithLabel(kube.RadixJobNameLabel, "job3").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(9))).
						WithCondition(v1.DeploymentInactive),
					getRadixDeploymentBuilderFor("dev", "rd2").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit2",
						}).
						WithLabel(kube.RadixJobNameLabel, "job2").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(5))).
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit3", "qa": ""},
		},
		{
			name:         "multiple envs deployments - get env-specific commit",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
				createPipelineJobs(appName, "job2", v1.BuildDeploy),
				createPipelineJobs(appName, "job3", v1.BuildDeploy),
				createPipelineJobs(appName, "job4", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd2-qa").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit2-dev",
						}).
						WithLabel(kube.RadixJobNameLabel, "job2").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(5))).
						WithCondition(v1.DeploymentActive),
					getRadixDeploymentBuilderFor("dev", "rd1-qa").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1-dev",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(1))).
						WithCondition(v1.DeploymentInactive),
				},
				"qa": {
					getRadixDeploymentBuilderFor("qa", "rd2-qa").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit2-qa",
						}).
						WithLabel(kube.RadixJobNameLabel, "job4").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(5))).
						WithCondition(v1.DeploymentActive),
					getRadixDeploymentBuilderFor("qa", "rd1-qa").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1-dev",
						}).
						WithLabel(kube.RadixJobNameLabel, "job3").
						WithActiveFrom(timeNow.Add(time.Hour * time.Duration(1))).
						WithCondition(v1.DeploymentInactive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit2-dev", "qa": "commit2-qa"},
		},
		{
			name:         "multiple deployments - gets commit for build-deploy pipeline job radix deployment",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.BuildDeploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "commit1", "qa": ""},
		},
		{
			name:         "multiple deployments - does not get commit for build pipeline job radix deployment",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.Build),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "", "qa": ""},
		},
		{
			name:         "multiple deployments - does not get commit for deploy pipeline job radix deployment",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.Deploy),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "", "qa": ""},
		},
		{
			name:         "multiple deployments - does not get commit for promote pipeline job radix deployment",
			environments: []string{"dev", "qa"},
			pipelineJobs: []*v1.RadixJob{
				createPipelineJobs(appName, "job1", v1.Promote),
			},
			envRadixDeploymentBuilders: map[string][]utils.DeploymentBuilder{
				"dev": {
					getRadixDeploymentBuilderFor("dev", "rd1").
						WithAnnotations(map[string]string{
							kube.RadixCommitAnnotation: "commit1",
						}).
						WithLabel(kube.RadixJobNameLabel, "job1").
						WithCondition(v1.DeploymentActive),
				},
			},
			expectedEnvCommits: map[string]string{"dev": "", "qa": ""},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			provider := prepareTestGetLastSuccessfulEnvironmentDeployCommits(t, appName, scenario)

			commits, err := provider.GetLastCommitHashesForEnvironments()

			assert.NoError(t, err)
			commitEnvs := maps.GetKeysFromMap(commits)
			assert.ElementsMatch(t, scenario.environments, commitEnvs)
			for _, env := range commitEnvs {
				assert.Equal(t, scenario.expectedEnvCommits[env], commits[env].CommitHash)
			}
		})
	}
}

func getRadixDeploymentBuilderFor(env string, deploymentName string) utils.DeploymentBuilder {
	return utils.ARadixDeployment().
		WithEnvironment(env).
		WithDeploymentName(deploymentName)
}

func createPipelineJobs(appName, jobName string, pipelineType v1.RadixPipelineType) *v1.RadixJob {
	return &v1.RadixJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: utils.GetAppNamespace(appName),
		},
		Spec: v1.RadixJobSpec{
			PipeLineType: pipelineType,
		},
	}
}

func prepareTestGetLastSuccessfulEnvironmentDeployCommits(t *testing.T, appName string, scenario scenarioGetLastSuccessfulEnvironmentDeployCommits) Provider {
	kubeClient, radixClient, _ := test.Setup()
	for _, environment := range scenario.environments {
		environmentNamespace := utils.GetEnvironmentNamespace(appName, environment)
		_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: environmentNamespace}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err.Error())
		}
		if radixDeploymentBuilders, ok := scenario.envRadixDeploymentBuilders[environment]; ok {
			for _, radixDeploymentBuilder := range radixDeploymentBuilders {
				_, err := radixClient.RadixV1().RadixDeployments(environmentNamespace).
					Create(context.Background(), radixDeploymentBuilder.WithAppName(appName).BuildRD(), metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err.Error())
				}
			}
		}
	}
	for _, pipelineJob := range scenario.pipelineJobs {
		_, _ = radixClient.RadixV1().RadixJobs(pipelineJob.GetNamespace()).Create(context.Background(), pipelineJob, metav1.CreateOptions{})
	}
	return NewProvider(kubeClient, radixClient, appName, scenario.environments)
}
