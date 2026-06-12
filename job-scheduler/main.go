package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/api/v1/controllers"
	batchControllers "github.com/equinor/radix-operator/job-scheduler/api/v1/controllers/batches"
	jobControllers "github.com/equinor/radix-operator/job-scheduler/api/v1/controllers/jobs"
	batchApi "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/batches"
	jobApi "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs"
	"github.com/equinor/radix-operator/job-scheduler/internal"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/models/common"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/job-scheduler/pkg/notifications"
	"github.com/equinor/radix-operator/job-scheduler/pkg/watcher"
	"github.com/equinor/radix-operator/job-scheduler/router"
	_ "github.com/equinor/radix-operator/job-scheduler/swaggerui"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const (
	defaultProfilePort = "7070"
)

const (
	concurrencyAllow   = "Allow"
	concurrencyForbid  = "Forbid"
	concurrencyReplace = "Replace"
)

type cronServer struct {
	kubeUtil     *kube.Kube
	env          *models.Env
	jobComponent *radixv1.RadixDeployJobComponent
	jobHandler   jobApi.JobHandler

	mu sync.Mutex
}

func (cs *cronServer) findActiveCronBatches(ctx context.Context) ([]*radixv1.RadixBatch, error) {
	jobName := cs.jobComponent.GetName()
	cronBatches, err := internal.GetRadixBatches(
		ctx,
		cs.kubeUtil.RadixClient(),
		cs.env.RadixDeploymentNamespace,
		labels.ForComponentName(jobName),
		labels.ForBatchCron(true),
	)
	if err != nil {
		return nil, err
	}

	activeCronBatches := make([]*radixv1.RadixBatch, 0)
	for _, b := range cronBatches {
		if b.Status.Condition.Type == radixv1.BatchConditionTypeActive {
			activeCronBatches = append(activeCronBatches, b)
		}
	}

	return activeCronBatches, nil
}

func (cs *cronServer) prepareForRun(ctx context.Context) (bool, error) {
	activeCronBatches, err := cs.findActiveCronBatches(ctx)
	if err != nil {
		return false, err
	}

	if len(activeCronBatches) == 0 {
		return true, nil
	}

	jobName := cs.jobComponent.GetName()
	switch cs.jobComponent.Cron.Concurrency {
	case concurrencyForbid:
		log.Info().Msgf("skipping cron job %s: an active batch is already running (Forbid)", jobName)
		return false, nil
	case concurrencyReplace:
		if err := cs.stopBatchJobs(ctx, activeCronBatches); err != nil {
			return false, fmt.Errorf("failed to stop active batch for cron job %s: %w", jobName, err)
		}

		log.Info().Msgf("stopped active batch(es) for cron job %s (Replace)", jobName)
		return true, nil
	case concurrencyAllow:
		return true, nil
	default:
		log.Warn().Msgf("invalid concurrency value detected for cron job %s", jobName)
		return false, nil
	}
}

func (cs *cronServer) start(ctx context.Context) error {
	tz, err := time.LoadLocation(cs.jobComponent.Cron.TimeZone)
	if err != nil {
		return fmt.Errorf("invalid timezone %q for cron job %s: %w",
			cs.jobComponent.Cron.TimeZone, cs.jobComponent.GetName(), err)
	}

	cronInstance := cron.New(cron.WithLocation(tz))

	for _, schedule := range cs.jobComponent.Cron.Schedule {
		if _, err := cronInstance.AddFunc(schedule, func() {
			cs.mu.Lock()
			defer cs.mu.Unlock()

			// Detach from ctx so a SIGTERM mid-callback cannot tear down the
			// stop-then-create sequence used by the Replace concurrency mode.
			runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()

			ok, err := cs.prepareForRun(runCtx)
			if err != nil {
				log.Err(err).Msg("failed to prepare cron run")
				return
			}
			if !ok {
				return
			}

			desc := &common.JobScheduleDescription{
				JobId: fmt.Sprintf("cron-%s", strings.ToLower(utils.RandString(8))),
				// Payload not supported
			}
			if _, err := cs.jobHandler.CreateJob(runCtx, desc, true); err != nil {
				log.Error().Err(err).Msg("failed to create scheduled job")
			}
		}); err != nil {
			return err
		}
	}

	cronInstance.Start()

	<-ctx.Done()
	<-cronInstance.Stop().Done()

	return nil
}

func (cs *cronServer) stopBatchJobs(ctx context.Context, batches []*radixv1.RadixBatch) error {
	for _, b := range batches {
		if err := batch.StopRadixBatchJob(
			ctx,
			cs.kubeUtil.RadixClient(),
			cs.env.RadixAppName,
			cs.env.RadixEnvironmentName,
			cs.jobComponent.Name,
			b.GetName(),
			"",
		); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	env := models.NewEnv()
	initLogger(env)

	kubeUtil := getKubeUtil()

	radixDeployJobComponent, err := getRadixDeployJobComponentByName(ctx, kubeUtil, env)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get job specification")
	}

	jobHistory := batch.NewHistory(kubeUtil, env, radixDeployJobComponent)
	notifier := notifications.NewWebhookNotifier(radixDeployJobComponent)
	radixBatchWatcher, err := watcher.NewRadixBatchWatcher(ctx, kubeUtil.RadixClient(), env.RadixDeploymentNamespace, jobHistory, notifier)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize job watcher")
	}
	defer radixBatchWatcher.Stop()

	jobHandler := jobApi.New(kubeUtil, env, radixDeployJobComponent)

	cs := cronServer{
		kubeUtil:     kubeUtil,
		env:          env,
		jobComponent: radixDeployJobComponent,
		jobHandler:   jobHandler,
	}

	g, gctx := errgroup.WithContext(ctx)
	if len(radixDeployJobComponent.Cron.Schedule) > 0 {
		g.Go(func() error {
			return cs.start(gctx)
		})
	}
	g.Go(func() error {
		runApiServer(gctx, jobHandler, kubeUtil, env, radixDeployJobComponent)
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Error().Err(err).Msg("server group exited with error")
	}
}

func initLogger(env *models.Env) {
	logLevelStr := env.LogLevel
	if len(logLevelStr) == 0 {
		logLevelStr = zerolog.LevelInfoValue
	}

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.DefaultContextLogger = &log.Logger
}

func runApiServer(ctx context.Context, jobHandler jobApi.JobHandler, kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) {
	fs := initializeFlagSet()
	port := fs.StringP("port", "p", env.RadixPort, "Port where API will be served")
	parseFlagsFromArgs(fs)

	var servers []*http.Server
	if env.UseProfiler {
		log.Info().Msgf("Initializing a profile server on a port %s", defaultProfilePort)
		servers = append(servers, &http.Server{Addr: fmt.Sprintf(":%s", defaultProfilePort)})
	}
	apiServer := &http.Server{
		Addr:        fmt.Sprintf(":%s", *port),
		Handler:     router.NewServer(env, getControllers(jobHandler, kubeUtil, env, radixDeployJobComponent)...),
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	servers = append(servers, apiServer)

	startServers(servers...)

	<-ctx.Done()
	shutdownServersGracefulOnSignal(servers...)
}

func startServers(servers ...*http.Server) {
	for _, srv := range servers {
		go func() {
			log.Debug().Msgf("Starting a server on address %s", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal().Err(err).Msgf("Unable to start server on address %s", srv.Addr)
				return
			}
			log.Info().Msgf("Started a server on address %s", srv.Addr)
		}()
	}
}

func getKubeUtil() *kube.Kube {
	kubeClient, radixClient, kedaClient, secretProviderClient, _, _ := utils.GetKubernetesClient()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretProviderClient)
	return kubeUtil
}

func getControllers(jobHandler jobApi.JobHandler, kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []controllers.Controller {
	return []controllers.Controller{
		jobControllers.New(jobHandler),
		batchControllers.New(batchApi.New(kubeUtil, env, radixDeployJobComponent)),
	}
}

func initializeFlagSet() *pflag.FlagSet {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "DESCRIPTION\n")
		fmt.Fprint(os.Stderr, "Radix job scheduler API server.\n")
		fmt.Fprint(os.Stderr, "\n")
		fmt.Fprint(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	return fs
}

func parseFlagsFromArgs(fs *pflag.FlagSet) {
	err := fs.Parse(os.Args[1:])
	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		log.Error().Err(err).Msg("Failed to parse flags")
		fs.Usage()
		os.Exit(2)
	}
}

func shutdownServersGracefulOnSignal(servers ...*http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info().Msgf("Shutting down server on address %s", srv.Addr)
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Warn().Err(err).Msgf("shutdown of server on address %s returned an error", srv.Addr)
			}
		}()
	}
	wg.Wait()
}

func getRadixDeployJobComponentByName(ctx context.Context, kube *kube.Kube, env *models.Env) (*radixv1.RadixDeployJobComponent, error) {
	rd, err := kube.GetRadixDeployment(ctx, env.RadixDeploymentNamespace, env.RadixDeploymentName)
	if err != nil {
		return nil, err
	}
	job, ok := slice.FindFirst(rd.Spec.Jobs, func(j radixv1.RadixDeployJobComponent) bool { return j.Name == env.RadixComponentName })
	if !ok {
		return nil, fmt.Errorf("job component %s does not exist in deployment %s", env.RadixComponentName, env.RadixDeploymentName)
	}
	return &job, nil
}
