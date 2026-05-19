package recovery

import (
	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
)

func NewMiddleware() *negroni.Recovery {
	rec := negroni.NewRecovery()
	rec.PrintStack = false
	rec.Logger = &log.Logger
	return rec
}
