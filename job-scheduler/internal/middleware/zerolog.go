package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
)

type SetZerologLoggerFn func(context.Context) zerolog.Logger

// ZerologLoggerWithRequestId returns a zerolog logger with a request_id field with a new GUID
func ZerologLoggerWithRequestId(ctx context.Context) zerolog.Logger {
	return zerolog.Ctx(ctx).With().Str("request_id", xid.New().String()).Logger()
}

// SetZerologLogger attaches the zerolog logger returned from each loggerFns function to a shallow copy of the gin request context
// The logger can then be accessed in a controller method by calling zerolog.Ctx(ctx)
func SetZerologLogger(loggerFns ...SetZerologLoggerFn) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, loggerFn := range loggerFns {
			logger := loggerFn(c.Request.Context())
			c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context()))
		}
		c.Next()
	}
}

// ZerologRequestLogger logs request and response using the zerolog logger from the request context
func ZerologRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())
		start := time.Now()

		c.Next()

		elapsed := time.Since(start)
		msg := http.StatusText(c.Writer.Status())
		if len(c.Errors) > 0 {
			msg = c.Errors.String()
		}

		var ev *zerolog.Event
		switch {
		case c.Writer.Status() >= 400 && c.Writer.Status() <= 499:
			ev = logger.Warn() //nolint:zerologlint
		case c.Writer.Status() >= 500:
			ev = logger.Error() //nolint:zerologlint
		default:
			ev = logger.Info() //nolint:zerologlint
		}

		ev.
			Str("remote_addr", c.ClientIP()).
			Str("referer", c.Request.Referer()).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Int("status", c.Writer.Status()).
			Int("body_size", c.Writer.Size()).
			Int64("elapsed_ms", elapsed.Milliseconds()).
			Str("user_agent", c.Request.UserAgent()).
			Msg(msg)
	}
}
