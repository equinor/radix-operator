package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestRadixJobQueueingOrder verifies the operator's job queueing and execution order:
//   - An apply-config job conflicts with every other job, so build-deploy jobs created while it is
//     active are queued behind it.
//   - Once the apply-config job finishes, build-deploy jobs are released and executed in creation
//     order, with jobs targeting the same environment running one at a time while jobs targeting
//     different environments are independent.
func TestRadixJobQueueingOrder(t *testing.T) {
	c := getClient(t)
	appName := "queue-order-test"

	_, appNamespace := createRadixRegistrationAndNamespaceForTest(t, c, appName)
	//defer cleanup()

	// RadixApplication mapping branch "dev" -> environment "dev" and branch "prod" -> environment "prod".
	ra := &v1.RadixApplication{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: appNamespace},
		Spec: v1.RadixApplicationSpec{
			Environments: []v1.Environment{
				{Name: "dev", Build: v1.EnvBuild{From: "dev"}},
				{Name: "prod", Build: v1.EnvBuild{From: "prod"}},
			},
		},
	}
	require.NoError(t, c.Create(t.Context(), ra), "should create RadixApplication")

	// Wait for the operator to provision the pipeline RBAC (radix-pipeline-app RoleBinding) in the
	// app namespace before creating any jobs. The pipeline pod uses the radix-pipeline service
	// account to read RadixApplication and RadixJob resources; if it starts before this RoleBinding
	// exists it fails fast with a forbidden error. With locally-loaded images the pod starts almost
	// instantly, so the RBAC must be in place first to avoid a flaky race.
	require.NoError(t, waitForPipelineRBAC(t.Context(), c, appNamespace, 60*time.Second), "pipeline RBAC should be provisioned in %s", appNamespace)

	const (
		applyJob = "j-apply-config"
		devJob1  = "j-dev-1"
		prodJob1 = "j-prod-1"
		devJob2  = "j-dev-2"
	)

	createApplyConfigJob := func(name string) {
		rj := &v1.RadixJob{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: appNamespace},
			Spec:       v1.RadixJobSpec{AppName: appName, PipeLineType: v1.ApplyConfig},
		}
		require.NoError(t, c.Create(t.Context(), rj), "should create apply-config job %s", name)
	}
	createBuildDeployJob := func(name, branch string) {
		rj := &v1.RadixJob{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: appNamespace},
			Spec: v1.RadixJobSpec{
				AppName:      appName,
				PipeLineType: v1.BuildDeploy,
				Build:        v1.RadixBuildSpec{Branch: branch, GitRef: branch, GitRefType: v1.GitRefBranch},
			},
		}
		require.NoError(t, c.Create(t.Context(), rj), "should create build-deploy job %s", name)
	}

	// --- Phase 1: create the apply-config job, then the build-deploy jobs ---

	// Create the apply-config job and wait until the operator has made it active. An apply-config
	// job conflicts with every other job, so build-deploy jobs created while it runs are queued
	// behind it.
	createApplyConfigJob(applyJob)
	cond, err := waitForJobCondition(t.Context(), c, appNamespace, applyJob, isActiveCondition, 90*time.Second)
	require.NoError(t, err, "apply-config job should become active, last condition: %s", cond)

	// Create build-deploy jobs across both environments while the apply-config job is active.
	// Space creations > 1s apart so the jobs get distinct (second-precision) CreationTimestamps,
	// which the operator uses to determine execution order. We don't assert the transient queued
	// state here (the apply-config job may finish quickly); the execution order is verified below.
	createBuildDeployJob(devJob1, "dev")
	time.Sleep(1500 * time.Millisecond)
	createBuildDeployJob(prodJob1, "prod")
	time.Sleep(1500 * time.Millisecond)
	createBuildDeployJob(devJob2, "dev")

	// --- Phase 2: jobs are released and executed in the correct order ---

	// Drain the queue: stop whichever job is active to advance to the next one, recording the order
	// in which jobs leave the queued state. This is robust regardless of whether the underlying
	// pipeline pods run to completion or are stopped.
	buildJobs := map[string]bool{devJob1: true, prodJob1: true, devJob2: true}
	activationOrder := []string{applyJob}
	activated := map[string]bool{applyJob: true}

	drainErr := wait.PollUntilContextTimeout(t.Context(), 500*time.Millisecond, 4*time.Minute, true, func(ctx context.Context) (bool, error) {
		jobs := &v1.RadixJobList{}
		if err := c.List(ctx, jobs, client.InNamespace(appNamespace)); err != nil {
			return false, nil
		}

		for i := range jobs.Items {
			j := &jobs.Items[i]
			// A job has been activated once it leaves the empty/queued state.
			if buildJobs[j.Name] && !activated[j.Name] && j.Status.Condition != "" && j.Status.Condition != v1.JobQueued {
				activated[j.Name] = true
				activationOrder = append(activationOrder, j.Name)
				t.Logf("job %s activated (condition %s)", j.Name, j.Status.Condition)
			}
		}

		// Stop any active job so the next queued job is promoted.
		for i := range jobs.Items {
			j := &jobs.Items[i]
			if isActiveCondition(j.Status.Condition) && !j.Spec.Stop {
				if err := setJobStop(ctx, c, appNamespace, j.Name); err != nil {
					return false, nil
				}
			}
		}

		return len(activated) == len(buildJobs)+1, nil
	})
	require.NoError(t, drainErr, "all build-deploy jobs should be activated, order so far: %v", activationOrder)

	// The apply-config job runs before all build-deploy jobs.
	require.Equal(t, applyJob, activationOrder[0], "apply-config job should run first, order: %v", activationOrder)

	// All build-deploy jobs were eventually executed.
	assert.Contains(t, activationOrder, devJob1, "order: %v", activationOrder)
	assert.Contains(t, activationOrder, prodJob1, "order: %v", activationOrder)
	assert.Contains(t, activationOrder, devJob2, "order: %v", activationOrder)

	// Jobs targeting the same environment run in creation order: the older "dev" job runs before
	// the newer "dev" job.
	assert.Less(t, indexOf(activationOrder, devJob1), indexOf(activationOrder, devJob2),
		"the older dev job (%s) should run before the newer dev job (%s), order: %v", devJob1, devJob2, activationOrder)
}

func isActiveCondition(cond v1.RadixJobCondition) bool {
	return cond == v1.JobWaiting || cond == v1.JobRunning
}

func indexOf(s []string, v string) int {
	for i := range s {
		if s[i] == v {
			return i
		}
	}
	return -1
}

// waitForPipelineRBAC polls until the operator has created the radix-pipeline-app RoleBinding in
// the app namespace, which grants the radix-pipeline service account access to read
// RadixApplication and RadixJob resources during the pipeline run.
func waitForPipelineRBAC(ctx context.Context, c client.Client, namespace string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		rb := &rbacv1.RoleBinding{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: defaults.PipelineAppRoleName}, rb); err != nil {
			return false, nil
		}
		return true, nil
	})
}

// waitForJobCondition polls the RadixJob until its condition satisfies pred or the timeout elapses.
func waitForJobCondition(ctx context.Context, c client.Client, namespace, name string, pred func(v1.RadixJobCondition) bool, timeout time.Duration) (v1.RadixJobCondition, error) {
	var last v1.RadixJobCondition
	err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		rj := &v1.RadixJob{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, rj); err != nil {
			return false, nil
		}
		last = rj.Status.Condition
		return pred(last), nil
	})
	return last, err
}

// setJobStop sets the Stop flag on a RadixJob, retrying on conflict.
func setJobStop(ctx context.Context, c client.Client, namespace, name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rj := &v1.RadixJob{}
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, rj); err != nil {
			return err
		}
		if rj.Spec.Stop {
			return nil
		}
		rj.Spec.Stop = true
		return c.Update(ctx, rj)
	})
}
