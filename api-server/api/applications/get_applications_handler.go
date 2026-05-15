package applications

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/equinor/radix-common/utils/slice"
	applicationModels "github.com/equinor/radix-operator/api-server/api/applications/models"
	"github.com/equinor/radix-operator/api-server/api/environments"
	environmentModels "github.com/equinor/radix-operator/api-server/api/environments/models"
	jobModels "github.com/equinor/radix-operator/api-server/api/jobs/models"
	"github.com/equinor/radix-operator/api-server/api/kubequery"
	"github.com/equinor/radix-operator/api-server/api/utils/access"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"golang.org/x/sync/errgroup"

	authorizationapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type hasAccessToRR func(ctx context.Context, client kubernetes.Interface, rr v1.RadixRegistration) (bool, error)

type GetApplicationsOptions struct {
	IncludeLatestJobSummary bool
	IncludeEnvironments     bool
}

// GetApplications handler for ShowApplications - NOTE: does not get latestJob.Environments
func (ah *ApplicationHandler) GetApplications(ctx context.Context, matcher applicationModels.ApplicationMatch, hasAccess hasAccessToRR, options GetApplicationsOptions) ([]*applicationModels.ApplicationSummary, error) {
	radixRegistationList, err := ah.getServiceAccount().RadixClient.RadixV1().RadixRegistrations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	filteredRegistrations := make([]v1.RadixRegistration, 0, len(radixRegistationList.Items))
	for _, rr := range radixRegistationList.Items {
		if matcher(&rr) {
			filteredRegistrations = append(filteredRegistrations, rr)
		}
	}

	radixRegistrations, err := ah.filterRadixRegByAccess(ctx, filteredRegistrations, hasAccess)
	if err != nil {
		return nil, err
	}

	var latestApplicationJobs map[string]*jobModels.JobSummary
	if options.IncludeLatestJobSummary {
		if latestApplicationJobs, err = getLatestJobPerApplication(ctx, ah.accounts.UserAccount.RadixClient, radixRegistrations); err != nil {
			return nil, err
		}
	}

	var appEnvironmentsMap map[string][]environmentModels.Environment
	if options.IncludeEnvironments {
		if appEnvironmentsMap, err = ah.getEnvironmentsForApplications(ctx, radixRegistrations); err != nil {
			return nil, err
		}
	}

	applications := make([]*applicationModels.ApplicationSummary, 0)
	for _, rr := range radixRegistrations {
		appName := rr.GetName()
		applications = append(
			applications,
			&applicationModels.ApplicationSummary{
				Name:         appName,
				LatestJob:    latestApplicationJobs[appName],
				Environments: appEnvironmentsMap[appName],
			},
		)
	}
	return applications, nil
}

func (ah *ApplicationHandler) getEnvironmentsForApplications(ctx context.Context, radixRegistrations []v1.RadixRegistration) (map[string][]environmentModels.Environment, error) {
	type ChannelData struct {
		key          string
		environments []environmentModels.Environment
	}

	var g errgroup.Group
	g.SetLimit(10)

	chanData := make(chan *ChannelData, len(radixRegistrations))
	for _, rr := range radixRegistrations {
		appName := rr.GetName()
		g.Go(func() error {
			reList, err := kubequery.GetRadixEnvironments(ctx, ah.accounts.ServiceAccount.RadixClient, appName)
			if err != nil {
				return err
			}

			envNames := slice.Map(reList, func(re v1.RadixEnvironment) string { return re.Spec.EnvName })

			environments, err := getEnvironmentsForApplication(ctx, ah.environmentHandler, appName, envNames)
			if err == nil {
				chanData <- &ChannelData{key: appName, environments: environments}
			}
			return err
		})
	}

	err := g.Wait()
	close(chanData)
	if err != nil {
		return nil, err
	}

	appEnvironments := make(map[string][]environmentModels.Environment)
	for data := range chanData {
		appEnvironments[data.key] = data.environments
	}
	return appEnvironments, nil
}

func getEnvironmentsForApplication(ctx context.Context, handler environments.EnvironmentHandler, appName string, envNames []string) ([]environmentModels.Environment, error) {
	var g errgroup.Group
	g.SetLimit(5)

	chanData := make(chan environmentModels.Environment, len(envNames))
	for _, envName := range envNames {

		g.Go(func() error {
			environmentModel, err := handler.GetEnvironment(ctx, appName, envName)
			if err == nil {
				chanData <- *environmentModel
			}
			return err
		})
	}

	err := g.Wait()
	close(chanData)
	if err != nil {
		return nil, err
	}

	environments := make([]environmentModels.Environment, 0, len(envNames))
	for data := range chanData {
		environments = append(environments, data)
	}

	slices.SortFunc(environments, func(a environmentModels.Environment, b environmentModels.Environment) int {
		return strings.Compare(a.Name, b.Name)
	})

	return environments, nil
}

func (ah *ApplicationHandler) filterRadixRegByAccess(ctx context.Context, radixregs []v1.RadixRegistration, hasAccess hasAccessToRR) ([]v1.RadixRegistration, error) {
	result := []v1.RadixRegistration{}
	limit := 25
	rrChan := make(chan v1.RadixRegistration, len(radixregs))
	kubeClient := ah.getUserAccount().Client
	var g errgroup.Group
	g.SetLimit(limit)

	checkAccess := func(rr v1.RadixRegistration) func() error {
		return func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if rr.Status.Reconciled.IsZero() {
				return nil
			}
			ok, err := hasAccess(ctx, kubeClient, rr)
			if ok {
				rrChan <- rr
			}
			return err
		}
	}

	for _, rr := range radixregs {
		g.Go(checkAccess(rr))
	}

	err := g.Wait()
	close(rrChan)
	if err != nil {
		return nil, err
	}

	for rr := range rrChan {
		result = append(result, rr)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.Compare(result[i].Name, result[j].Name) == -1
	})
	return result, nil
}

// cannot run as test - does not return correct values
func hasAccess(ctx context.Context, client kubernetes.Interface, rr v1.RadixRegistration) (bool, error) {
	return access.HasAccess(ctx, client, &authorizationapi.ResourceAttributes{
		Verb:     "get",
		Group:    "radix.equinor.com",
		Resource: "radixregistrations",
		Version:  "*",
		Name:     rr.GetName(),
	})
}

func getLatestJobPerApplication(ctx context.Context, radixClient versioned.Interface, radixRegistations []v1.RadixRegistration) (map[string]*jobModels.JobSummary, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(25)
	jobSummaries := sync.Map{}

	for _, rr := range radixRegistations {
		g.Go(func() error {
			jobs, err := kubequery.GetRadixJobs(ctx, radixClient, rr.GetName())
			if err != nil {
				return err
			}

			var latestJob *v1.RadixJob
			for _, job := range jobs {
				if job.Status.Started == nil {
					continue
				}

				if latestJob == nil || job.Status.Started.After(latestJob.Status.Started.Time) {
					latestJob = &job
				}
			}

			if latestJob != nil {
				jobSummaries.Store(rr.GetName(), jobModels.GetSummaryFromRadixJob(latestJob))
			}
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	applicationJob := make(map[string]*jobModels.JobSummary, len(radixRegistations))
	for _, rr := range radixRegistations {
		job, _ := jobSummaries.Load(rr.GetName())
		if job != nil {
			applicationJob[rr.GetName()] = job.(*jobModels.JobSummary)
		}
	}

	return applicationJob, nil
}
