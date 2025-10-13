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
	"sync"
	"syscall"
	"time"

	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/job-scheduler/api/v1/controllers"
	batchControllers "github.com/equinor/radix-operator/job-scheduler/api/v1/controllers/batches"
	jobControllers "github.com/equinor/radix-operator/job-scheduler/api/v1/controllers/jobs"
	batchApi "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/batches"
	jobApi "github.com/equinor/radix-operator/job-scheduler/api/v1/handlers/jobs"
	"github.com/equinor/radix-operator/job-scheduler/models"
	"github.com/equinor/radix-operator/job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/job-scheduler/pkg/notifications"
	"github.com/equinor/radix-operator/job-scheduler/pkg/watcher"
	"github.com/equinor/radix-operator/job-scheduler/router"
	_ "github.com/equinor/radix-operator/job-scheduler/swaggerui"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
)

const (
	defaultProfilePort = "7070"
)

func main() {
	ctx := context.Background()
	env := models.NewEnv()
	initLogger(env)

	kubeUtil := getKubeUtil(ctx)

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

	runApiServer(ctx, kubeUtil, env, radixDeployJobComponent)
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

func runApiServer(ctx context.Context, kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		Handler:     router.NewServer(env, getControllers(kubeUtil, env, radixDeployJobComponent)...),
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

func getKubeUtil(ctx context.Context) *kube.Kube {
	kubeClient, radixClient, kedaClient, _, secretProviderClient, _, _ := utils.GetKubernetesClient(ctx)
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretProviderClient)
	return kubeUtil
}

func getControllers(kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []controllers.Controller {
	return []controllers.Controller{
		jobControllers.New(jobApi.New(kubeUtil, env, radixDeployJobComponent)),
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
