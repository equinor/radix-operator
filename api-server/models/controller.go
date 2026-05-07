package models

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/rs/zerolog/log"
)

// RadixHandlerFunc Pattern for handler functions
type RadixHandlerFunc func(Accounts, http.ResponseWriter, *http.Request)

// Controller Pattern of an rest/stream controller
type Controller interface {
	GetRoutes() Routes
}

// DefaultController Default implementation
type DefaultController struct {
}

// ErrorResponse Marshals error for user requester
func (c *DefaultController) ErrorResponse(w http.ResponseWriter, r *http.Request, err error) {

	logError(r.Context(), err)
	err = radixhttp.ErrorResponse(w, r, err)
	if err != nil {
		log.Ctx(r.Context()).Error().Err(err).Msg("failed to write response")
	}
}

func logError(ctx context.Context, err error) {
	event := log.Ctx(ctx).Warn().Err(err)

	var httpErr *radixhttp.Error
	if errors.As(err, &httpErr) {
		event.Str("user_message", httpErr.Message)
	}

	event.Msg("controller error")
}

// JSONResponse Marshals response with header
func (c *DefaultController) JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	err := radixhttp.JSONResponse(w, r, result)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}

// ReaderFileResponse writes the content from the reader to the response,
// and sets Content-Disposition=attachment; filename=<filename arg>
func (c *DefaultController) ReaderFileResponse(w http.ResponseWriter, r *http.Request, reader io.Reader, fileName, contentType string) {
	err := radixhttp.ReaderFileResponse(w, reader, fileName, contentType)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}

// ReaderResponse writes the content from the reader to the response,
func (c *DefaultController) ReaderResponse(w http.ResponseWriter, r *http.Request, reader io.Reader, contentType string) {
	err := radixhttp.ReaderResponse(w, reader, contentType)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}

// ReaderEventStreamResponse writes the content from the reader to the response, one line at a time as an event stream. Will stop at the end of stream, or when the client disconnects.
// Every 15 seconds a healthcheck event is sent to keep the connection alive.
func (c *DefaultController) ReaderEventStreamResponse(w http.ResponseWriter, r *http.Request, reader io.Reader) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Ctx(r.Context()).Err(errors.New("streaming unsupported")).Msg("failed to write response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering for nginx
	w.WriteHeader(http.StatusOK)
	m := sync.Mutex{}

	// Make sure we send an initial message to the client
	_, _ = fmt.Fprintf(w, "event: started\n\n") // sends an event comment
	flusher.Flush()

	// send health checks every 5 seconds
	tickerCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.Lock()
				_, _ = fmt.Fprintf(w, ": healthcheck\n\n") // sends an event comment
				flusher.Flush()
				m.Unlock()
			case <-tickerCtx.Done():
				return
			}
		}
	}()

	// Loop over lines and send them with "data: " prefix
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		m.Lock()
		_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
		m.Unlock()
	}

	m.Lock()
	defer m.Unlock() // good practice

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		log.Ctx(r.Context()).Err(err).Msg("failed to read stream")
		_, _ = fmt.Fprintf(w, "event: error\n\n")
		return
	}

	_, _ = fmt.Fprintf(w, "event: completed\n\n")
}

// ByteArrayResponse Used for response data. I.e. image
func (c *DefaultController) ByteArrayResponse(w http.ResponseWriter, r *http.Request, contentType string, result []byte) {
	err := radixhttp.ByteArrayResponse(w, r, contentType, result)
	if err != nil {
		log.Ctx(r.Context()).Err(err).Msg("failed to write response")
	}
}
