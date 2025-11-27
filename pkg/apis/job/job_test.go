package job

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/config"
	"github.com/equinor/radix-operator/pkg/apis/config/containerregistry"
	"github.com/equinor/radix-operator/pkg/apis/config/dnsalias"
	"github.com/equinor/radix-operator/pkg/apis/config/pipelinejob"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/test"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/annotations"
	radix "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	secretproviderfake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type RadixJobTestSuiteBase struct {
	suite.Suite
	testUtils   *test.Utils
	kubeClient  *kubernetes.Clientset
	kubeUtils   *kube.Kube
	radixClient *radix.Clientset
	config      struct {
		clusterName    string
		builderImage   string
		buildkitImage  string
		buildahSecComp string
		gitImage       string
		radixZone      string
		clusterType    string
		registry       string
		appRegistry    string
		subscriptionID string
	}
}

func (s *RadixJobTestSuiteBase) SetupSuite() {
	s.config = struct {
		clusterName    string
		builderImage   string
		buildkitImage  string
		buildahSecComp string
		gitImage       string
		radixZone      string
		clusterType    string
		registry       string
		appRegistry    string
		subscriptionID string
	}{
		clusterName:    "AnyClusterName",
		builderImage:   "docker.io/builder:any",
		buildkitImage:  "docker.io/buildkit:any",
		buildahSecComp: "anyseccomp",
		gitImage:       "docker.io/git:any",
		radixZone:      "anyzone",
		clusterType:    "anyclustertype",
		registry:       "anyregistry",
		appRegistry:    "anyAppRegistry",
		subscriptionID: "anysubid",
	}
}

func (s *RadixJobTestSuiteBase) SetupTest() {
	s.setupTest()
}

func (s *RadixJobTestSuiteBase) setupTest() {
	// Setup
	kubeClient := kubernetes.NewSimpleClientset()
	radixClient := radix.NewSimpleClientset()
	kedaClient := kedafake.NewSimpleClientset()
	secretproviderclient := secretproviderfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretproviderclient)
	handlerTestUtils := test.NewTestUtils(kubeClient, radixClient, kedaClient, secretproviderclient)
	err := handlerTestUtils.CreateClusterPrerequisites(s.config.clusterName, s.config.subscriptionID)
	s.Require().NoError(err)
	s.testUtils, s.kubeClient, s.kubeUtils, s.radixClient = &handlerTestUtils, kubeClient, kubeUtil, radixClient

	s.T().Setenv(defaults.OperatorClusterTypeEnvironmentVariable, s.config.clusterType)
	s.T().Setenv(defaults.RadixZoneEnvironmentVariable, s.config.radixZone)
	s.T().Setenv(defaults.ContainerRegistryEnvironmentVariable, s.config.registry)
	s.T().Setenv(defaults.AppContainerRegistryEnvironmentVariable, s.config.appRegistry)
	s.T().Setenv(defaults.RadixImageBuilderEnvironmentVariable, s.config.builderImage)
	s.T().Setenv(defaults.RadixBuildKitImageBuilderEnvironmentVariable, s.config.buildkitImage)
	s.T().Setenv(defaults.SeccompProfileFileNameEnvironmentVariable, s.config.buildahSecComp)
	s.T().Setenv(defaults.RadixGitCloneGitImageEnvironmentVariable, s.config.gitImage)
}

func (s *RadixJobTestSuiteBase) applyJobWithSync(regBuilder utils.RegistrationBuilder, jobBuilder utils.JobBuilder, config *config.Config) (*radixv1.RadixJob, *radixv1.RadixRegistration, error) {
	rj, err := s.testUtils.ApplyJob(jobBuilder)
	if err != nil {
		return nil, nil, err
	}

	rr, err := s.testUtils.ApplyRegistration(regBuilder)
	if err != nil {
		return nil, nil, err
	}

	err = s.runSync(rr, rj, config)
	if err != nil {
		return nil, nil, err
	}

	newRj, err := s.radixClient.RadixV1().RadixJobs(rj.GetNamespace()).Get(context.Background(), rj.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	newRr, err := s.radixClient.RadixV1().RadixRegistrations().Get(context.Background(), rr.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	return newRj, newRr, nil
}

func (s *RadixJobTestSuiteBase) runSync(rr *radixv1.RadixRegistration, rj *radixv1.RadixJob, config *config.Config) error {
	job := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rr, rj, config)
	return job.OnSync(context.Background())
}

func TestRadixJobTestSuite(t *testing.T) {
	suite.Run(t, new(RadixJobTestSuite))
}

type RadixJobTestSuite struct {
	RadixJobTestSuiteBase
}

func (s *RadixJobTestSuite) SetupSubTest() {
	s.setupTest()
}

func (s *RadixJobTestSuite) Test_ReconcileStatus() {
	qty := resource.MustParse("1")
	cfg := &config.Config{
		PipelineJobConfig: &pipelinejob.Config{
			AppBuilderResourcesLimitsCPU:      &qty,
			AppBuilderResourcesLimitsMemory:   &qty,
			AppBuilderResourcesRequestsCPU:    &qty,
			AppBuilderResourcesRequestsMemory: &qty,
		},
		DNSConfig: &dnsalias.DNSConfig{},
	}
	rr := &radixv1.RadixRegistration{}
	rj := &radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "any-name", Generation: 42}}
	rj, err := s.radixClient.RadixV1().RadixJobs("any-ns").Create(context.Background(), rj, metav1.CreateOptions{})
	s.Require().NoError(err)

	// First sync sets status
	expectedGen := rj.Generation
	sut := NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rr, rj, cfg)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	rj, err = s.radixClient.RadixV1().RadixJobs(rj.Namespace).Get(context.Background(), rj.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixJobReconcileSucceeded, rj.Status.ReconcileStatus)
	s.Empty(rj.Status.Message)
	s.Equal(expectedGen, rj.Status.ObservedGeneration)
	s.False(rj.Status.Reconciled.IsZero())

	// Second sync with updated generation
	rj.Generation++
	expectedGen = rj.Generation
	sut = NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rr, rj, cfg)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	rj, err = s.radixClient.RadixV1().RadixJobs(rj.Namespace).Get(context.Background(), rj.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixJobReconcileSucceeded, rj.Status.ReconcileStatus)
	s.Empty(rj.Status.Message)
	s.Equal(expectedGen, rj.Status.ObservedGeneration)
	s.False(rj.Status.Reconciled.IsZero())

	// Sync with stop
	rjStop := &radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "stop-job", Generation: 20}, Spec: radixv1.RadixJobSpec{Stop: true}}
	rjStop, err = s.radixClient.RadixV1().RadixJobs("any-ns").Create(context.Background(), rjStop, metav1.CreateOptions{})
	s.Require().NoError(err)
	expectedGen = rjStop.Generation
	sut = NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rr, rjStop, cfg)
	err = sut.OnSync(context.Background())
	s.Require().NoError(err)
	rjStop, err = s.radixClient.RadixV1().RadixJobs(rjStop.Namespace).Get(context.Background(), rjStop.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixJobReconcileSucceeded, rjStop.Status.ReconcileStatus)
	s.Empty(rj.Status.Message)
	s.Equal(expectedGen, rjStop.Status.ObservedGeneration)
	s.False(rjStop.Status.Reconciled.IsZero())

	// Sync with error
	rjErr := &radixv1.RadixJob{ObjectMeta: metav1.ObjectMeta{Name: "err-job", Generation: 10}}
	rjErr, err = s.radixClient.RadixV1().RadixJobs("any-ns").Create(context.Background(), rjErr, metav1.CreateOptions{})
	s.Require().NoError(err)
	errorMsg := "any sync error"
	s.kubeClient.PrependReactor("create", "jobs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New(errorMsg)
	})
	expectedGen = rjErr.Generation
	sut = NewJob(s.kubeClient, s.kubeUtils, s.radixClient, rr, rjErr, cfg)
	err = sut.OnSync(context.Background())
	s.Require().ErrorContains(err, errorMsg)
	rjErr, err = s.radixClient.RadixV1().RadixJobs(rjErr.Namespace).Get(context.Background(), rjErr.Name, metav1.GetOptions{})
	s.Require().NoError(err)
	s.Equal(radixv1.RadixJobReconcileFailed, rjErr.Status.ReconcileStatus)
	s.Contains(rjErr.Status.Message, errorMsg)
	s.Equal(expectedGen, rjErr.Status.ObservedGeneration)
	s.False(rjErr.Status.Reconciled.IsZero())
}

func (s *RadixJobTestSuite) Test_QueuedJob_ReconcileStatus() {
	config := getConfigWithPipelineJobsHistoryLimit(3)

	// Setup
	_, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)))
	s.Require().NoError(err)

	// Sync job -> queued
	expectedGen := int64(42)
	secondJob, _, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithGeneration(expectedGen).WithJobName("SecondJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)
	s.Equal(radixv1.RadixJobReconcileSucceeded, secondJob.Status.ReconcileStatus)
	s.Empty(secondJob.Status.Message)
	s.Equal(expectedGen, secondJob.Status.ObservedGeneration)
	s.False(secondJob.Status.Reconciled.IsZero())
}

func (s *RadixJobTestSuite) TestObjectSynced_PipelineJobCreated() {
	appID := ulid.Make()
	appName, jobName, gitRef, gitRefType, envName, deploymentName, commitID, imageTag, pipelineTag := "anyapp", "anyjobname", "anytag", string(radixv1.GitRefTag), "anyenv", "anydeploy", "anycommit", "anyimagetag", "docker.io/anypipeline:tag"
	config := getConfigWithPipelineJobsHistoryLimit(3)
	rj, _, err := s.applyJobWithSync(
		utils.NewRegistrationBuilder().WithName(appName).WithAppID(appID.String()).WithRadixConfigFullName("some-radixconfig.yaml"),
		utils.NewJobBuilder().
			WithJobName(jobName).
			WithAppName(appName).
			WithGitRef(gitRef).
			WithGitRefType(gitRefType).
			WithToEnvironment(envName).
			WithCommitID(commitID).
			WithPushImage(true).
			WithImageTag(imageTag).
			WithDeploymentName(deploymentName).
			WithPipelineType(radixv1.BuildDeploy),
		config)
	s.Require().NoError(err)
	jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
	s.Require().Len(jobs.Items, 1)
	job := jobs.Items[0]
	s.Equal(GetOwnerReference(rj), job.OwnerReferences)
	expectedJobLabels := map[string]string{kube.RadixJobNameLabel: jobName, "radix-pipeline": string(radixv1.BuildDeploy), kube.RadixJobTypeLabel: kube.RadixJobTypeJob, kube.RadixAppLabel: appName, kube.RadixCommitLabel: commitID, kube.RadixImageTagLabel: imageTag}
	s.Equal(expectedJobLabels, job.Labels)
	expectedJobAnnotations := map[string]string{kube.RadixBranchAnnotation: "", kube.RadixGitRefAnnotation: gitRef, kube.RadixGitRefTypeAnnotation: gitRefType}
	s.Equal(expectedJobAnnotations, job.Annotations)
	podTemplate := job.Spec.Template
	s.Equal(annotations.ForClusterAutoscalerSafeToEvict(false), podTemplate.Annotations)

	s.Equal(corev1.RestartPolicyNever, podTemplate.Spec.RestartPolicy)
	expectedTolerations := []corev1.Toleration{{Key: kube.NodeTaintJobsKey, Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists}}
	s.ElementsMatch(expectedTolerations, podTemplate.Spec.Tolerations)
	expectedAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: kube.RadixJobNodeLabel, Operator: corev1.NodeSelectorOpExists},
		{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{defaults.DefaultNodeSelectorOS}},
		{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(radixv1.RuntimeArchitectureArm64)}},
	}}}}}}
	s.Equal(expectedAffinity, podTemplate.Spec.Affinity)
	expectedSecurityCtx := &corev1.PodSecurityContext{FSGroup: pointers.Ptr[int64](1000), RunAsNonRoot: pointers.Ptr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}
	s.Equal(expectedSecurityCtx, podTemplate.Spec.SecurityContext)

	expectedContainers := []corev1.Container{
		{
			Name:            "radix-pipeline",
			Image:           pipelineTag,
			ImagePullPolicy: corev1.PullAlways,
			Args: []string{
				fmt.Sprintf("--RADIX_APP=%s", appName),
				fmt.Sprintf("--JOB_NAME=%s", jobName),
				fmt.Sprintf("--PIPELINE_TYPE=%s", radixv1.BuildDeploy),
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_MEMORY=1000Mi",
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_REQUESTS_CPU=100m",
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_MEMORY=2000Mi",
				"--RADIXOPERATOR_APP_BUILDER_RESOURCES_LIMITS_CPU=200m",
				fmt.Sprintf("--RADIX_EXTERNAL_REGISTRY_DEFAULT_AUTH_SECRET=%s", config.ContainerRegistryConfig.ExternalRegistryAuthSecret),
				fmt.Sprintf("--RADIX_IMAGE_BUILDER_IMAGE=%s", s.config.builderImage),
				fmt.Sprintf("--RADIX_BUILDKIT_IMAGE_BUILDER_IMAGE=%s", s.config.buildkitImage),
				fmt.Sprintf("--SECCOMP_PROFILE_FILENAME=%s", s.config.buildahSecComp),
				fmt.Sprintf("--RADIX_CLUSTER_TYPE=%s", s.config.clusterType),
				fmt.Sprintf("--RADIX_ZONE=%s", s.config.radixZone),
				fmt.Sprintf("--RADIX_CLUSTERNAME=%s", s.config.clusterName),
				fmt.Sprintf("--RADIX_CONTAINER_REGISTRY=%s", s.config.registry),
				fmt.Sprintf("--RADIX_APP_CONTAINER_REGISTRY=%s", s.config.appRegistry),
				fmt.Sprintf("--AZURE_SUBSCRIPTION_ID=%s", s.config.subscriptionID),
				"--RADIX_GITHUB_WORKSPACE=/workspace",
				"--RADIX_FILE_NAME=some-radixconfig.yaml",
				"--TRIGGERED_FROM_WEBHOOK=false",
				fmt.Sprintf("--RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE=%s", s.config.gitImage),
				fmt.Sprintf("--IMAGE_TAG=%s", imageTag),
				"--BRANCH=",
				fmt.Sprintf("--GIT_REF=%s", gitRef),
				fmt.Sprintf("--GIT_REF_TYPE=%s", gitRefType),
				fmt.Sprintf("--TO_ENVIRONMENT=%s", envName),
				fmt.Sprintf("--COMMIT_ID=%s", commitID),
				"--PUSH_IMAGE=1",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "build-context",
					MountPath: "/workspace",
					ReadOnly:  false,
				},
				{
					Name:      "pod-labels",
					MountPath: "/pod-labels",
					ReadOnly:  false,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("2000Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("250Mi"),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged:               pointers.Ptr(false),
				AllowPrivilegeEscalation: pointers.Ptr(false),
				RunAsNonRoot:             pointers.Ptr(true),
				ReadOnlyRootFilesystem:   pointers.Ptr(true),
				RunAsUser:                pointers.Ptr[int64](1000),
				RunAsGroup:               pointers.Ptr[int64](1000),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
		},
	}

	expectedInitContainers := []corev1.Container{
		{
			Name:            "clone-config",
			Image:           s.config.gitImage,
			Command:         []string{"sh", "-c", "umask 002 && git config --global --add safe.directory /workspace && git clone  -b  --verbose --progress /workspace && (git submodule update --init --recursive || echo \"Warning: Unable to clone submodules, proceeding without them\") && chmod -R g+r /workspace/.git"},
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env:             []corev1.EnvVar{{Name: "HOME", Value: "/home/clone"}},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "build-context",
					MountPath: "/workspace",
					ReadOnly:  false,
				},
				{
					Name:      "git-ssh-keys",
					MountPath: "/.ssh",
					ReadOnly:  true,
				},
				{
					Name:      "builder-home",
					MountPath: "/home/clone",
					ReadOnly:  false,
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(250, resource.Mega),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(1000, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(2000, resource.Mega),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				RunAsNonRoot:             pointers.Ptr(true),
				RunAsUser:                pointers.Ptr[int64](65534),
				RunAsGroup:               pointers.Ptr[int64](1000),
				SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				ReadOnlyRootFilesystem:   pointers.Ptr(true),
				AllowPrivilegeEscalation: pointers.Ptr(false),
				Privileged:               pointers.Ptr(false),
				ProcMount:                nil,
			},
		},
	}
	expectedVolumes := []corev1.Volume{
		{Name: "build-context"},
		{Name: "builder-home"},
		{Name: "git-ssh-keys", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "git-ssh-keys", DefaultMode: pointers.Ptr[int32](256)}}},
		{Name: "pod-labels", VolumeSource: corev1.VolumeSource{DownwardAPI: &corev1.DownwardAPIVolumeSource{Items: []corev1.DownwardAPIVolumeFile{{Path: "labels", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"}}}}}},
	}

	expectedPodLabels := map[string]string{kube.RadixJobNameLabel: jobName, kube.RadixAppLabel: "anyapp", kube.RadixAppIDLabel: appID.String()}
	s.Equal(expectedPodLabels, podTemplate.Labels)
	expectedPodAnnotations := annotations.ForClusterAutoscalerSafeToEvict(false)
	s.Equal(expectedPodAnnotations, podTemplate.Annotations)
	expectedPodSpec := corev1.PodSpec{
		ImagePullSecrets:   []corev1.LocalObjectReference{{Name: "an-external-registry-secret"}},
		RestartPolicy:      corev1.RestartPolicyNever,
		Tolerations:        expectedTolerations,
		Affinity:           expectedAffinity,
		ServiceAccountName: "radix-pipeline",
		SecurityContext:    expectedSecurityCtx,
		Containers:         expectedContainers,
		Volumes:            expectedVolumes,
	}

	actualInitContainers := podTemplate.Spec.InitContainers
	podTemplate.Spec.InitContainers = nil

	s.Equal(expectedPodSpec, podTemplate.Spec)
	s.Require().Equal(len(expectedInitContainers), len(actualInitContainers))
	for i := range expectedInitContainers {
		s.Equal(expectedInitContainers[i], actualInitContainers[i], "init container %s not equal", expectedInitContainers[i].Name)
	}

}

func (s *RadixJobTestSuite) TestObjectSynced_BuildKit() {
	const appName, jobName, branch, envName, deploymentName, commitID, imageTag, pipelineTag = "anyapp", "anyjobname", "anybranch", "anyenv", "anydeploy", "anycommit", "anyimagetag", "docker.io/anypipeline:tag"
	argUseBuildCacheTrue := fmt.Sprintf("--%s=true", defaults.RadixOverrideUseBuildCacheEnvironmentVariable)
	argUseBuildCacheFalse := fmt.Sprintf("--%s=false", defaults.RadixOverrideUseBuildCacheEnvironmentVariable)
	argRefreshBuildCacheTrue := fmt.Sprintf("--%s=true", defaults.RadixRefreshBuildCacheEnvironmentVariable)
	argRefreshBuildCacheFalse := fmt.Sprintf("--%s=false", defaults.RadixRefreshBuildCacheEnvironmentVariable)

	scenarios := map[string]struct {
		build                            *radixv1.BuildSpec
		overrideUseBuildCache            *bool
		refreshBuildCache                *bool
		expectedArgOverrideUseBuildCache string
		expectedArgRefreshBuildCache     string
	}{
		"Build empty": {
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, overrideUseBuildCache true": {
			overrideUseBuildCache:            pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, overrideUseBuildCache false": {
			overrideUseBuildCache:            pointers.Ptr(false),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, overrideUseBuildCache true, refreshBuildCache false": {
			overrideUseBuildCache:            pointers.Ptr(true),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, overrideUseBuildCache false, refreshBuildCache true": {
			overrideUseBuildCache:            pointers.Ptr(false),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, refreshBuildCache false": {
			overrideUseBuildCache:            pointers.Ptr(true),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build empty, refreshBuildCache true": {
			overrideUseBuildCache:            pointers.Ptr(false),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty": {
			build:                            &radixv1.BuildSpec{},
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, overrideUseBuildCache true": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, overrideUseBuildCache false": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(false),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, overrideUseBuildCache true, refreshBuildCache false": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(true),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, overrideUseBuildCache false, refreshBuildCache true": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(false),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, refreshBuildCache false": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(true),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"Build not empty, refreshBuildCache true": {
			build:                            &radixv1.BuildSpec{},
			overrideUseBuildCache:            pointers.Ptr(false),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit, overrideUseBuildCache true": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			overrideUseBuildCache:            pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: argUseBuildCacheTrue,
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit, overrideUseBuildCache false": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			overrideUseBuildCache:            pointers.Ptr(false),
			expectedArgOverrideUseBuildCache: argUseBuildCacheFalse,
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit, overrideUseBuildCache true, refreshBuildCache true": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			overrideUseBuildCache:            pointers.Ptr(true),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: argUseBuildCacheTrue,
			expectedArgRefreshBuildCache:     argRefreshBuildCacheTrue,
		},
		"UseBuildKit, overrideUseBuildCache false, refreshBuildCache true": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			overrideUseBuildCache:            pointers.Ptr(false),
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: argUseBuildCacheFalse,
			expectedArgRefreshBuildCache:     argRefreshBuildCacheTrue,
		},
		"UseBuildKit, implicit UseBuildCache": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true)},
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit, explicite UseBuildCache, explicite refreshBuildCache": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(true)},
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     "",
		},
		"UseBuildKit, implicit UseBuildCache, explicite no refreshBuildCache": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(false)},
			refreshBuildCache:                pointers.Ptr(false),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     argRefreshBuildCacheFalse,
		},
		"UseBuildKit, explicite UseBuildCache, explicite refreshBuildCache, refreshBuildCache": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(true)},
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     argRefreshBuildCacheTrue,
		},
		"UseBuildKit, explicite no UseBuildCache, explicite refreshBuildCache": {
			build:                            &radixv1.BuildSpec{UseBuildKit: pointers.Ptr(true), UseBuildCache: pointers.Ptr(false)},
			refreshBuildCache:                pointers.Ptr(true),
			expectedArgOverrideUseBuildCache: "",
			expectedArgRefreshBuildCache:     argRefreshBuildCacheTrue,
		},
	}

	for name, scenario := range scenarios {
		s.T().Run(name, func(t *testing.T) {
			s.T().Log(name)
			s.SetupTest()
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), utils.NewRegistrationBuilder().WithName(appName).WithRadixConfigFullName("some-radixconfig.yaml").BuildRR(), metav1.CreateOptions{})
			s.Require().NoError(err)
			applicationBuilder := utils.ARadixApplication()
			if scenario.build != nil {
				applicationBuilder = applicationBuilder.WithBuildKit(scenario.build.UseBuildKit).WithBuildCache(scenario.build.UseBuildCache)
			}
			_, _, err = s.applyJobWithSync(
				utils.ARadixRegistration().WithName(appName),
				utils.NewJobBuilder().
					WithJobName(jobName).
					WithRadixApplication(applicationBuilder).
					WithAppName(appName).
					WithGitRef(branch).
					WithGitRefType(string(radixv1.GitRefBranch)).
					WithToEnvironment(envName).
					WithCommitID(commitID).
					WithPushImage(true).
					WithImageTag(imageTag).
					WithDeploymentName(deploymentName).
					WithPipelineType(radixv1.BuildDeploy).
					WithOverrideUseBuildCache(scenario.overrideUseBuildCache).
					WithRefreshBuildCache(scenario.refreshBuildCache),
				getConfigWithPipelineJobsHistoryLimit(3))
			s.Require().NoError(err)
			jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(jobs.Items, 1)
			job := jobs.Items[0]

			if len(scenario.expectedArgOverrideUseBuildCache) > 0 {
				s.Contains(job.Spec.Template.Spec.Containers[0].Args, scenario.expectedArgOverrideUseBuildCache, "expected argument for UseBuildCache %s not found in job args", scenario.expectedArgOverrideUseBuildCache)
			} else {
				arg := fmt.Sprintf("--%s=", defaults.RadixOverrideUseBuildCacheEnvironmentVariable)
				s.NotContains(job.Spec.Template.Spec.Containers[0].Args, arg, "unexpected expected argument for UseBuildCache %s not found in job args", arg)
			}

			if len(scenario.expectedArgRefreshBuildCache) > 0 {
				s.Contains(job.Spec.Template.Spec.Containers[0].Args, scenario.expectedArgRefreshBuildCache, "expected argument for RefreshBuildCache %s not found in job args", scenario.expectedArgRefreshBuildCache)
			} else {
				arg := fmt.Sprintf("--%s=", defaults.RadixRefreshBuildCacheEnvironmentVariable)
				s.NotContains(job.Spec.Template.Spec.Containers[0].Args, arg, "unexpected expected argument for RefreshBuildCache %s not found in job args", arg)
			}

		})
	}
}

func (s *RadixJobTestSuite) TestObjectSynced_GitCloneArguments() {
	var (
		appName = "anyapp"
		jobName = "anyjobname"
		gitArg  = fmt.Sprintf("--RADIX_PIPELINE_GIT_CLONE_GIT_IMAGE=%s", s.config.gitImage)
		config  = getConfigWithPipelineJobsHistoryLimit(3)
	)
	tests := map[string]struct {
		unsetGit     bool
		expectedArgs []string
	}{
		"all set":   {false, []string{gitArg}},
		"git unset": {true, []string{}},
	}

	for name, test := range tests {
		s.Run(name, func() {
			if test.unsetGit {
				s.T().Setenv(defaults.RadixGitCloneGitImageEnvironmentVariable, "")
			}
			_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), utils.NewRegistrationBuilder().WithName(appName).BuildRR(), metav1.CreateOptions{})
			s.Require().NoError(err)
			_, _, err = s.applyJobWithSync(
				utils.ARadixRegistration().WithName(appName),
				utils.NewJobBuilder().
					WithJobName(jobName).
					WithAppName(appName).
					WithPipelineType(radixv1.BuildDeploy),
				config)
			s.Require().NoError(err)
			jobs, _ := s.kubeClient.BatchV1().Jobs(utils.GetAppNamespace(appName)).List(context.Background(), metav1.ListOptions{})
			s.Require().Len(jobs.Items, 1)
			s.Require().Len(jobs.Items[0].Spec.Template.Spec.Containers, 1)
			actualArgs := slice.FindAll(jobs.Items[0].Spec.Template.Spec.Containers[0].Args, func(arg string) bool {
				return strings.HasPrefix(arg, "--RADIX_PIPELINE_GIT_CLONE")
			})
			s.Subset(actualArgs, test.expectedArgs)
		})
	}
}

func (s *RadixJobTestSuite) TestObjectSynced_FirstJobRunning_SecondJobQueued() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)))
	s.Require().NoError(err)

	// Test
	secondJob, rr, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	_, err = s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.Background(), firstJob, metav1.UpdateOptions{})
	s.Require().NoError(err)

	err = s.runSync(rr, firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.Background(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_FirstJobWaiting_SecondJobQueued() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.ARadixBuildDeployJob().WithStatus(utils.NewJobStatusBuilder().WithCondition(radixv1.JobWaiting)).WithJobName("FirstJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)))
	s.Require().NoError(err)

	// Test
	secondJob, rr, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	_, err = s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.Background(), firstJob, metav1.UpdateOptions{})
	s.Require().NoError(err)

	err = s.runSync(rr, firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.Background(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobs_MissingRadixApplication() {
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), utils.NewRegistrationBuilder().WithName("some-app").BuildRR(), metav1.CreateOptions{})
	s.Require().NoError(err)
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	firstJob, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithRadixApplication(nil).WithJobName("FirstJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)))
	s.Require().NoError(err)

	// Test
	secondJob, _, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("SecondJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobQueued, secondJob.Status.Condition)

	// Third job differen branch
	thirdJob, rr, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("ThirdJob").WithBranch("qa"), config)
	s.Require().NoError(err)
	s.Equal(radixv1.JobWaiting, thirdJob.Status.Condition)

	// Stopping first job should set second job to running
	firstJob.Spec.Stop = true
	_, err = s.radixClient.RadixV1().RadixJobs(firstJob.ObjectMeta.Namespace).Update(context.Background(), firstJob, metav1.UpdateOptions{})
	s.Require().NoError(err)

	err = s.runSync(rr, firstJob, config)
	s.Require().NoError(err)

	secondJob, _ = s.radixClient.RadixV1().RadixJobs(secondJob.ObjectMeta.Namespace).Get(context.Background(), secondJob.Name, metav1.GetOptions{})
	s.Equal(radixv1.JobRunning, secondJob.Status.Condition)
}

func (s *RadixJobTestSuite) TestObjectSynced_MultipleJobsDifferentBranch_SecondJobRunning() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	// Setup
	_, err := s.testUtils.ApplyJob(utils.AStartedBuildDeployJob().WithJobName("FirstJob").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)))
	s.Require().NoError(err)

	// Test
	secondJob, _, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithJobName("SecondJob").WithBranch("release"), config)
	s.Require().NoError(err)

	s.Equal(radixv1.JobWaiting, secondJob.Status.Condition)
}

type radixDeploymentJob struct {
	// jobName Radix pipeline job
	jobName string
	// rdName RadixDeployment name
	rdName string
	// env of environments, where RadixDeployments are deployed
	env string
	// branch to build
	branch string
	// type of the pipeline
	pipelineType radixv1.RadixPipelineType
	// jobStatus Status of the job
	jobStatus radixv1.RadixJobCondition
}

func (s *RadixJobTestSuite) TestHistoryLimit_EachEnvHasOwnHistory() {
	appName := "anyApp"
	appNamespace := utils.GetAppNamespace(appName)
	const envDev = "dev"
	const envQa = "qa"
	const envProd = "prod"
	const branchDevQa = "main"
	const branchProd = "release"
	rrBuilder := utils.ARadixRegistration().WithName(appName)
	raBuilder := utils.ARadixApplication().
		WithRadixRegistration(rrBuilder).
		WithEnvironment(envDev, branchDevQa).
		WithEnvironment(envQa, branchDevQa).
		WithEnvironment(envProd, branchProd)

	type jobScenario struct {
		// existingRadixDeploymentJobs List of RadixDeployments and its RadixJobs, setup before test
		existingRadixDeploymentJobs []radixDeploymentJob
		// testingRadixDeploymentJob RadixDeployments and its RadixJobs under test
		testingRadixDeploymentJob radixDeploymentJob
		// jobsHistoryLimitPerBranchAndStatus Limit, defined in the env-var   RADIX_PIPELINE_JOBS_HISTORY_LIMIT
		jobsHistoryLimit int
		// expectedJobNames Name of jobs, expected to exist on finishing test
		expectedJobNames []string
	}

	scenarios := map[string]jobScenario{
		"All jobs are successful and running - no deleted job": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobRunning,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		"All jobs are successful and queued - no deleted job": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobQueued,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		"All jobs are successful and waiting - no deleted job": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobWaiting,
			},
			expectedJobNames: []string{"j1", "j2", "j3"},
		},
		"Stopped job within the limit - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		"Stopped job out of the limit - old deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		"Failed job within the limit - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		"Failed job out of the limit - old deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		"StoppedNoChanges job within the limit - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2"},
		},
		"StoppedNoChanges job out of the limit - old deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j3", "j4"},
		},
		"Stopped and failed jobs within the limit - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5"},
		},
		"Stopped job out of the limit - old stopped deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6"},
		},
		"Failed job out of the limit - old falsed deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2", "j4", "j5", "j6"},
		},
		"StoppedNoChanges job out of the limit - old stopped-no-changes deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j5", "j6"},
		},
		"Failed job is within the limit on env - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobFailed},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		"Failed job out of the limit on env - old failed deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobFailed},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobFailed},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobFailed},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobFailed},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobFailed,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
		"Stopped job is within the limit on env - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStopped},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		"Stopped job out of the limit on env - old stopped deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStopped},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStopped},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStopped},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobStopped},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobStopped,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
		"StoppedNoChanges job is within the limit on env - not deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j6", rdName: "rd6", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j7", rdName: "rd7", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7"},
		},
		"StoppedNoChanges job out of the limit on env - old StoppedNoChanges deleted": {
			jobsHistoryLimit: 2,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", rdName: "rd1", env: envDev, jobStatus: radixv1.JobSucceeded},
				{jobName: "j2", rdName: "rd2", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j3", rdName: "rd3", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j4", rdName: "rd4", env: envQa, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j5", rdName: "rd5", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j6", rdName: "rd6", env: envDev, jobStatus: radixv1.JobStoppedNoChanges},
				{jobName: "j7", rdName: "rd7", env: envProd, jobStatus: radixv1.JobStoppedNoChanges},
			},
			testingRadixDeploymentJob: radixDeploymentJob{
				jobName: "j8", rdName: "rd8", env: envDev, jobStatus: radixv1.JobStoppedNoChanges,
			},
			expectedJobNames: []string{"j1", "j3", "j4", "j5", "j6", "j7", "j8"},
		},
	}

	for name, scenario := range scenarios {
		s.Run(name, func() {
			config := getConfigWithPipelineJobsHistoryLimit(scenario.jobsHistoryLimit)
			testTime := time.Now().Add(time.Hour * -100)
			for _, rdJob := range scenario.existingRadixDeploymentJobs {
				_, err := s.testUtils.ApplyDeployment(context.Background(), utils.ARadixDeployment().
					WithAppName(appName).WithDeploymentName(rdJob.rdName).WithEnvironment(rdJob.env).WithJobName(rdJob.jobName).
					WithActiveFrom(testTime))
				s.NoError(err)
				err = s.applyJobWithSyncFor(rrBuilder, raBuilder, appName, rdJob, config)
				s.Require().NoError(err)

				testTime = testTime.Add(time.Hour)
			}

			_, err := s.testUtils.ApplyDeployment(context.Background(), utils.ARadixDeployment().
				WithAppName(appName).WithDeploymentName(scenario.testingRadixDeploymentJob.rdName).
				WithEnvironment(scenario.testingRadixDeploymentJob.env).
				WithActiveFrom(testTime))
			s.NoError(err)
			err = s.applyJobWithSyncFor(rrBuilder, raBuilder, appName, scenario.testingRadixDeploymentJob, config)
			s.Require().NoError(err)

			radixJobList, err := s.radixClient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
			s.NoError(err)
			s.assertExistRadixJobsWithNames(radixJobList, scenario.expectedJobNames)
		})
	}
}

type jobConditions map[string]radixv1.RadixJobCondition

func (s *RadixJobTestSuite) Test_WildCardJobs() {
	appName := "anyApp"
	appNamespace := utils.GetAppNamespace(appName)
	const (
		envTest            = "test"
		envQa              = "qa"
		branchQa           = "qa"
		branchTest         = "test"
		branchTestWildCard = "test*"
		branchTest1        = "test1"
		branchTest2        = "test2"
	)

	type jobScenario struct {
		// name Scenario name
		raBuilder utils.ApplicationBuilder
		// existingRadixDeploymentJobs List of RadixDeployments and its RadixJobs, setup before test
		existingRadixDeploymentJobs []radixDeploymentJob
		// testingRadixJobBuilder RadixDeployments and its RadixJobs under test
		testingRadixJobBuilder utils.JobBuilder
		// expectedJobConditions State of jobs, mapped by job name, expected to exist on finishing test
		expectedJobConditions jobConditions
	}

	scenarios := map[string]jobScenario{
		"One job is running": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: nil,
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j-new": radixv1.JobWaiting},
		},
		"One job is running, new is queuing on same branch with one env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobQueued},
		},
		"One job is running, new is running on another branch with two envs": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchQa),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobWaiting},
		},
		"One job is running, new is queuing on same branch with wildcard": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTestWildCard),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest1, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest2),
			expectedJobConditions: jobConditions{"j1": radixv1.JobRunning, "j-new": radixv1.JobQueued},
		},
		"Multiple non-running, new is running on same branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTestWildCard),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest1, jobStatus: radixv1.JobSucceeded, pipelineType: radixv1.BuildDeploy},
				{jobName: "j2", env: envTest, branch: branchTest1, jobStatus: radixv1.JobFailed, pipelineType: radixv1.BuildDeploy},
				{jobName: "j3", env: envTest, branch: branchTest1, jobStatus: radixv1.JobStopped, pipelineType: radixv1.BuildDeploy},
				{jobName: "j4", env: envTest, branch: branchTest1, jobStatus: radixv1.JobStoppedNoChanges, pipelineType: radixv1.BuildDeploy},
				{jobName: "j5", env: envTest, branch: branchTest1, jobStatus: radixv1.JobQueued, pipelineType: radixv1.BuildDeploy},
				{jobName: "j6", env: envTest, branch: branchTest1, jobStatus: radixv1.JobWaiting, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest1),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobSucceeded,
				"j2":    radixv1.JobFailed,
				"j3":    radixv1.JobStopped,
				"j4":    radixv1.JobStoppedNoChanges,
				"j5":    radixv1.JobQueued,
				"j6":    radixv1.JobWaiting,
				"j-new": radixv1.JobQueued,
			},
		},
	}

	for name, scenario := range scenarios {
		s.Run(name, func() {
			appId := ulid.Make().String()
			config := getConfigWithPipelineJobsHistoryLimit(10)
			testTime := time.Now().Add(time.Hour * -100)
			rrBuilder := utils.ARadixRegistration().WithName(appName)
			rr, err := s.testUtils.ApplyRegistration(rrBuilder)
			s.Require().NoError(err)
			raBuilder := scenario.raBuilder.WithAppName(appName)
			_, err = s.testUtils.ApplyApplication(raBuilder)
			s.Require().NoError(err)

			for _, rdJob := range scenario.existingRadixDeploymentJobs {
				if rdJob.jobStatus == radixv1.JobSucceeded {
					_, err := s.testUtils.ApplyDeployment(context.Background(), utils.ARadixDeployment().
						WithRadixApplication(utils.ARadixApplication().WithRadixRegistration(utils.ARadixRegistration().WithAppID(appId))).
						WithAppName(appName).
						WithDeploymentName(fmt.Sprintf("%s-deployment", rdJob.jobName)).
						WithJobName(rdJob.jobName).
						WithActiveFrom(testTime))
					s.Require().NoError(err)
				}
				rjBuilder := utils.NewJobBuilder().WithAppName(appName).WithPipelineType(rdJob.pipelineType).
					WithJobName(rdJob.jobName).WithBranch(rdJob.branch).WithCreated(testTime.Add(time.Minute * 2)).
					WithStatus(utils.NewJobStatusBuilder().WithCondition(rdJob.jobStatus))
				_, err := s.testUtils.ApplyJob(rjBuilder)
				s.Require().NoError(err)
				testTime = testTime.Add(time.Hour)
			}

			testingRadixJob, err := s.testUtils.ApplyJob(scenario.testingRadixJobBuilder.WithAppName(appName))
			s.Require().NoError(err)
			err = s.runSync(rr, testingRadixJob, config)
			s.NoError(err)

			radixJobList, err := s.radixClient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.assertExistRadixJobsWithConditions(radixJobList, scenario.expectedJobConditions)
		})
	}
}

func (s *RadixJobTestSuite) Test_MultipleJobsForSameEnv() {
	appName := "anyApp"
	appNamespace := utils.GetAppNamespace(appName)
	const (
		envTest            = "test"
		envQa              = "qa"
		branchQa           = "qa"
		branchTest         = "test"
		branchTestWildCard = "test*"
		branchTest1        = "test1"
		branchTest2        = "test2"
	)

	type jobScenario struct {
		// name Scenario name
		raBuilder utils.ApplicationBuilder
		// existingRadixDeploymentJobs List of RadixDeployments and its RadixJobs, setup before test
		existingRadixDeploymentJobs []radixDeploymentJob
		// testingRadixJobBuilder RadixDeployments and its RadixJobs under test
		testingRadixJobBuilder utils.JobBuilder
		// expectedJobConditions State of jobs, mapped by job name, expected to exist on finishing test
		expectedJobConditions jobConditions
	}

	scenarios := map[string]jobScenario{
		"Single build-deploy job is running": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: nil,
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j-new": radixv1.JobWaiting},
		},
		"One deploy job is running, another build deploy is queuing for the same env by the branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Deploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One promote job is running, another build deploy is queuing for the same env by the branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Promote},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One deploy job is running, another promote is queuing for the same env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Deploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Promote).
				WithJobName("j-new").WithToEnvironment(envTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One promote job is running, another deploy is queuing for the same env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Promote},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Deploy).
				WithJobName("j-new").WithToEnvironment(envTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One build-deploy job is running, another promote is queuing for the same env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Promote).
				WithJobName("j-new").WithToEnvironment(envTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One build-deploy job is running, another deploy is queuing for the same env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Deploy).
				WithJobName("j-new").WithToEnvironment(envTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
			},
		},
		"One deploy job is running, another build deploy is starting for the different env by the branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Deploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One promote job is running, another build deploy is starting for the different env by the branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Promote},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One deploy job is running, another promote is starting for the different env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Deploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Promote).
				WithJobName("j-new").WithToEnvironment(envQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One promote job is running, another deploy is starting for the different env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.Promote},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Deploy).
				WithJobName("j-new").WithToEnvironment(envQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One build-deploy job is running, another promote is starting for the different env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Promote).
				WithJobName("j-new").WithToEnvironment(envQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One build-deploy job is running, another deploy is starting for the different env": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchQa),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.Deploy).
				WithJobName("j-new").WithToEnvironment(envQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
		"One build-deploy job is running, another build deploy is queuing for the same branch": {
			raBuilder: getRadixApplicationBuilder(appName).
				WithEnvironment(envTest, branchTest).
				WithEnvironment(envQa, branchTest),
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobQueued,
				// here it can be new job for the env envQa, when one job per environment is merged
			},
		},
		"Single build-deploy job is running without existing radix-app": {
			raBuilder:                   nil,
			existingRadixDeploymentJobs: nil,
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchTest),
			expectedJobConditions: jobConditions{"j-new": radixv1.JobWaiting},
		},
		"One build-deploy job is running, another build deploy is starting for the different branch without existing radix-app": {
			raBuilder: nil,
			existingRadixDeploymentJobs: []radixDeploymentJob{
				{jobName: "j1", env: envTest, branch: branchTest, jobStatus: radixv1.JobRunning, pipelineType: radixv1.BuildDeploy},
			},
			testingRadixJobBuilder: utils.NewJobBuilder().WithPipelineType(radixv1.BuildDeploy).
				WithJobName("j-new").WithBranch(branchQa),
			expectedJobConditions: jobConditions{
				"j1":    radixv1.JobRunning,
				"j-new": radixv1.JobWaiting,
			},
		},
	}

	for name, scenario := range scenarios {
		s.Run(name, func() {
			rr, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), utils.NewRegistrationBuilder().WithName(appName).BuildRR(), metav1.CreateOptions{})
			s.Require().NoError(err)
			config := getConfigWithPipelineJobsHistoryLimit(10)
			testTime := time.Now().Add(time.Hour * -100)
			if scenario.raBuilder != nil {
				raBuilder := scenario.raBuilder.WithAppName(appName)
				_, err := s.testUtils.ApplyApplication(raBuilder)
				s.Require().NoError(err)
			}

			for _, rdJob := range scenario.existingRadixDeploymentJobs {
				if rdJob.jobStatus == radixv1.JobSucceeded {
					_, err := s.testUtils.ApplyDeployment(context.Background(), utils.ARadixDeployment().
						WithRadixApplication(nil).
						WithAppName(appName).
						WithDeploymentName(fmt.Sprintf("%s-deployment", rdJob.jobName)).
						WithJobName(rdJob.jobName).
						WithActiveFrom(testTime))
					s.Require().NoError(err)
				}
				rjBuilder := utils.NewJobBuilder().WithAppName(appName).WithPipelineType(rdJob.pipelineType).
					WithJobName(rdJob.jobName).WithBranch(rdJob.branch).WithToEnvironment(rdJob.env).
					WithCreated(testTime.Add(time.Minute * 2)).
					WithStatus(utils.NewJobStatusBuilder().WithCondition(rdJob.jobStatus))
				_, err := s.testUtils.ApplyJob(rjBuilder)
				s.Require().NoError(err)
				testTime = testTime.Add(time.Hour)
			}

			testingRadixJob, err := s.testUtils.ApplyJob(scenario.testingRadixJobBuilder.WithAppName(appName))
			s.Require().NoError(err)
			err = s.runSync(rr, testingRadixJob, config)
			s.NoError(err)

			radixJobList, err := s.radixClient.RadixV1().RadixJobs(appNamespace).List(context.Background(), metav1.ListOptions{})
			s.Require().NoError(err)
			s.assertExistRadixJobsWithConditions(radixJobList, scenario.expectedJobConditions)
		})
	}
}

func getRadixApplicationBuilder(appName string) utils.ApplicationBuilder {
	return utils.NewRadixApplicationBuilder().
		WithRadixRegistration(utils.NewRegistrationBuilder().WithName(appName))
}
func (s *RadixJobTestSuite) assertExistRadixJobsWithConditions(radixJobList *radixv1.RadixJobList, expectedJobConditions jobConditions) {
	resultJobConditions := make(jobConditions)
	for _, rj := range radixJobList.Items {
		rj := rj
		resultJobConditions[rj.GetName()] = rj.Status.Condition
		if _, ok := expectedJobConditions[rj.GetName()]; !ok {
			s.Fail(fmt.Sprintf("unexpected job exists %s", rj.GetName()))
		}
	}
	for expectedJobName, expectedJobCondition := range expectedJobConditions {
		resultJobCondition, ok := resultJobConditions[expectedJobName]
		s.True(ok, "missing job %s", expectedJobName)
		s.Equal(expectedJobCondition, resultJobCondition, "unexpected job condition")
	}
}

func (s *RadixJobTestSuite) assertExistRadixJobsWithNames(radixJobList *radixv1.RadixJobList, expectedJobNames []string) {
	resultJobNames := make(map[string]bool)
	for _, rj := range radixJobList.Items {
		resultJobNames[rj.GetName()] = true
	}
	for _, jobName := range expectedJobNames {
		_, ok := resultJobNames[jobName]
		s.True(ok, "missing job %s", jobName)
	}
}

func (s *RadixJobTestSuite) applyJobWithSyncFor(rrBuilder utils.RegistrationBuilder, raBuilder utils.ApplicationBuilder, appName string, rdJob radixDeploymentJob, config *config.Config) error {
	_, _, err := s.applyJobWithSync(
		rrBuilder,
		utils.ARadixBuildDeployJob().
			WithRadixApplication(raBuilder).
			WithAppName(appName).
			WithJobName(rdJob.jobName).
			WithDeploymentName(rdJob.rdName).
			WithBranch(rdJob.env).
			WithStatus(utils.NewJobStatusBuilder().
				WithCondition(rdJob.jobStatus)),
		config)
	return err
}

func (s *RadixJobTestSuite) TestTargetEnvironmentIsSetWhenRadixApplicationExist() {
	config := getConfigWithPipelineJobsHistoryLimit(3)

	expectedEnvs := []string{"test"}
	job, _, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithJobName("test").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	// Master maps to Test env
	s.Equal(job.Spec.Build.GetGitRefOrDefault(), "master")
	s.Equal(expectedEnvs, job.Status.TargetEnvs)
}

func (s *RadixJobTestSuite) TestTargetEnvironmentEmptyWhenRadixApplicationMissing() {
	config := getConfigWithPipelineJobsHistoryLimit(3)
	_, err := s.radixClient.RadixV1().RadixRegistrations().Create(context.Background(), utils.NewRegistrationBuilder().WithName("some-app").BuildRR(), metav1.CreateOptions{})
	s.Require().NoError(err)

	job, _, err := s.applyJobWithSync(utils.ARadixRegistration(), utils.ARadixBuildDeployJob().WithRadixApplication(nil).WithJobName("test").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), config)
	s.Require().NoError(err)
	// Master maps to Test env
	s.Equal(job.Spec.Build.GetGitRefOrDefault(), "master")
	s.Empty(job.Status.TargetEnvs)
}

func (s *RadixJobTestSuite) TestObjectSynced_UseBuildKid_HasResourcesArgs() {
	scenarios := map[string]struct {
		config                                    *config.Config
		expectedAppBuilderResourcesRequestsCPU    string
		expectedAppBuilderResourcesRequestsMemory string
		expectedAppBuilderResourcesLimitsMemory   string
		expectedAppBuilderResourcesLimitsCPU      string
		expectedError                             string
	}{
		"Configured AppBuilderResources": {
			config: &config.Config{
				DNSZone: "dev.radix.equinor.com",
				PipelineJobConfig: &pipelinejob.Config{
					PipelineJobsHistoryLimit:          3,
					AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("123m")),
					AppBuilderResourcesLimitsCPU:      pointers.Ptr(resource.MustParse("456m")),
					AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
					AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2345Mi")),
					PipelineImage:                     "docker.io/anypipeline:tag",
					GitCloneImage:                     "docker.io/git:any",
				},
			},
			expectedError:                             "",
			expectedAppBuilderResourcesRequestsCPU:    "123m",
			expectedAppBuilderResourcesRequestsMemory: "1234Mi",
			expectedAppBuilderResourcesLimitsMemory:   "2345Mi",
			expectedAppBuilderResourcesLimitsCPU:      "456m",
		},
		"Missing config for ResourcesRequestsCPU": {
			config: &config.Config{
				DNSZone: "dev.radix.equinor.com",
				PipelineJobConfig: &pipelinejob.Config{
					AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
					AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2345Mi")),
					PipelineImage:                     "docker.io/anypipeline:tag",
					GitCloneImage:                     "docker.io/git:any",
				}},
			expectedError: "invalid or missing app builder resources",
		},
		"Missing config for ResourcesRequestsMemory": {
			config: &config.Config{
				DNSZone: "dev.radix.equinor.com",
				PipelineJobConfig: &pipelinejob.Config{
					AppBuilderResourcesRequestsCPU:  pointers.Ptr(resource.MustParse("123m")),
					AppBuilderResourcesLimitsMemory: pointers.Ptr(resource.MustParse("2345Mi")),
					PipelineImage:                   "docker.io/anypipeline:tag",
					GitCloneImage:                   "docker.io/git:any",
				}},
			expectedError: "invalid or missing app builder resources",
		},
		"Missing config for ResourcesLimitsMemory": {
			config: &config.Config{
				DNSZone: "dev.radix.equinor.com",
				PipelineJobConfig: &pipelinejob.Config{
					AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("123m")),
					AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1234Mi")),
					PipelineImage:                     "docker.io/anypipeline:tag",
					GitCloneImage:                     "docker.io/git:any",
				}},
			expectedError: "invalid or missing app builder resources",
		},
	}
	for name, scenario := range scenarios {
		s.Run(name, func() {
			_, _, err := s.applyJobWithSync(
				utils.ARadixRegistration(),
				utils.ARadixBuildDeployJobWithAppBuilder(func(m utils.ApplicationBuilder) {
					m.WithBuildKit(pointers.Ptr(true))
				}).WithJobName("job1").WithGitRef("master").WithGitRefType(string(radixv1.GitRefBranch)), scenario.config)
			switch {
			case len(scenario.expectedError) > 0 && err == nil:
				s.Fail(fmt.Sprintf("Missing expected error '%s'", scenario.expectedError))
				return
			case len(scenario.expectedError) == 0 && err != nil:
				s.Fail(fmt.Sprintf("Unexpected error %v", err))
				return
			case len(scenario.expectedError) > 0 && err != nil:
				s.Equal(scenario.expectedError, err.Error(), fmt.Sprintf("Expected error '%s' but got '%s'", scenario.expectedError, err.Error()))
				return
			}
			s.Require().NoError(err)

			jobList, err := s.testUtils.GetKubeUtil().ListJobs(context.Background(), utils.GetAppNamespace("some-app"))
			s.Require().NoError(err)

			s.Len(jobList, 1)
			job := jobList[0]
			s.Equal(scenario.expectedAppBuilderResourcesRequestsCPU, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesRequestsCPUEnvironmentVariable), "Invalid or missing AppBuilderResourcesRequestsCPU")
			s.Equal(scenario.expectedAppBuilderResourcesRequestsMemory, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesRequestsMemoryEnvironmentVariable), "Invalid or missing AppBuilderResourcesRequestsMemory")
			s.Equal(scenario.expectedAppBuilderResourcesLimitsMemory, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesLimitsMemoryEnvironmentVariable), "Invalid or missing AppBuilderResourcesLimitsMemory")
			s.Equal(scenario.expectedAppBuilderResourcesLimitsCPU, getJobContainerArgument(job.Spec.Template.Spec.Containers[0], defaults.OperatorAppBuilderResourcesLimitsCPUEnvironmentVariable), "Invalid or missing AppBuilderResourcesLimitsCPU")
		})

	}
}

func getJobContainerArgument(container corev1.Container, variableName string) string {
	for _, arg := range container.Args {
		argPrefix := fmt.Sprintf("--%s=", variableName)
		if strings.HasPrefix(arg, argPrefix) {
			return arg[len(argPrefix):]
		}
	}
	return ""
}

func getConfigWithPipelineJobsHistoryLimit(historyLimit int) *config.Config {
	return &config.Config{
		DNSZone: "dev.radix.equinor.com",
		PipelineJobConfig: &pipelinejob.Config{
			PipelineJobsHistoryLimit:          historyLimit,
			AppBuilderResourcesLimitsMemory:   pointers.Ptr(resource.MustParse("2000Mi")),
			AppBuilderResourcesLimitsCPU:      pointers.Ptr(resource.MustParse("200m")),
			AppBuilderResourcesRequestsCPU:    pointers.Ptr(resource.MustParse("100m")),
			AppBuilderResourcesRequestsMemory: pointers.Ptr(resource.MustParse("1000Mi")),
			PipelineImage:                     "docker.io/anypipeline:tag",
			GitCloneImage:                     "docker.io/git:any",
		},
		ContainerRegistryConfig: containerregistry.Config{
			ExternalRegistryAuthSecret: "an-external-registry-secret",
		},
	}
}
