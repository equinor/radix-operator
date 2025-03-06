package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitializeLogger(logLevel zerolog.Level, prettyPrint bool) {
	zerolog.SetGlobalLevel(logLevel)
	zerolog.DurationFieldUnit = time.Millisecond
	if prettyPrint {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	zerolog.DefaultContextLogger = &log.Logger
	log.Debug().Msgf("log-level '%v'", logLevel.String())
}
