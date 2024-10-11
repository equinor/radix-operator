package internal

import (
	"context"

	"github.com/rs/zerolog/log"
)

// TaskLogger Interface for logging tasks
type TaskLogger interface {
	// Info logs routine messages about cron's operation.
	Info(msg string, keysAndValues ...interface{})
	// Error logs an error condition.
	Error(err error, msg string, keysAndValues ...interface{})
}

type taskLogger struct {
	ctx context.Context
}

func (t taskLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Ctx(t.ctx).Info().Msgf(msg, keysAndValues...)
}

func (t taskLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Ctx(t.ctx).Err(err).Msgf(msg, keysAndValues...)
}

// NewLogger Creates a new task logger
func NewLogger(ctx context.Context) TaskLogger {
	return &taskLogger{ctx: ctx}
}
