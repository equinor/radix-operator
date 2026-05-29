package task

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	minOrphanedEnvironmentsRetentionPeriod = time.Minute * 5
)

// Config for tasks
type Config struct {
	// OrphanedRadixEnvironmentsRetentionPeriod is the time period for how long orphaned RadixEnvironments should be retained
	OrphanedRadixEnvironmentsRetentionPeriod time.Duration `envconfig:"RADIXOPERATOR_ORPHANED_ENVIRONMENTS_RETENTION_PERIOD" default:"720h"`
	// OrphanedEnvironmentsCleanupCron is the cron expression for when to run the cleanup of orphaned RadixEnvironments
	OrphanedEnvironmentsCleanupCron string `envconfig:"RADIXOPERATOR_ORPHANED_ENVIRONMENTS_CLEANUP_CRON" default:"0 0 * * *"`
}

func MustParseConfig() *Config {
	var tc Config
	if err := envconfig.Process("", &tc); err != nil {
		_ = envconfig.Usage("", &tc)
		log.Fatal().Msg(err.Error())
	}
	if tc.OrphanedRadixEnvironmentsRetentionPeriod < minOrphanedEnvironmentsRetentionPeriod {
		log.Warn().Msgf("RADIXOPERATOR_ORPHANED_ENVIRONMENTS_RETENTION_PERIOD must be at least %s. Set to maximum value", minOrphanedEnvironmentsRetentionPeriod)
		tc.OrphanedRadixEnvironmentsRetentionPeriod = minOrphanedEnvironmentsRetentionPeriod
	}
	if _, err := cron.ParseStandard(tc.OrphanedEnvironmentsCleanupCron); err != nil {
		log.Fatal().Msgf("RADIXOPERATOR_ORPHANED_ENVIRONMENTS_CLEANUP_CRON is not a valid cron expression: %v", err)
	}
	return &tc
}
