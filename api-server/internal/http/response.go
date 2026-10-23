package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/elnormous/contenttype"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var supportedMediaTypes []contenttype.MediaType = []contenttype.MediaType{
	contenttype.NewMediaType("application/json"),
	contenttype.NewMediaType("text/plain"),
}

// StringResponse Used for textual response data. I.e. log data
func StringResponse(w http.ResponseWriter, r *http.Request, result string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(result))
	return err
}

// ByteArrayResponse Used for response data. I.e. image
func ByteArrayResponse(w http.ResponseWriter, r *http.Request, contentType string, result []byte) error {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(result)
	return err
}

// JSONResponse Marshals response with header
func JSONResponse(w http.ResponseWriter, r *http.Request, result any) error {
	body, err := json.Marshal(result)
	if err != nil {
		return ErrorResponse(w, r, err)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	return err
}

// ReaderFileResponse writes the content from the reader to the response,
// and sets Content-Disposition=attachment; filename=<filename arg>
func ReaderFileResponse(w http.ResponseWriter, reader io.Reader, fileName, contentType string) error {
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", contentType)
	_, err := io.Copy(w, reader)
	return err
}

// ReaderResponse writes the content from the reader to the response,
func ReaderResponse(w http.ResponseWriter, reader io.Reader, contentType string) error {
	w.Header().Set("Content-Type", contentType)
	_, err := io.Copy(w, reader)
	return err
}

// ErrorResponse Marshals error for user requester
func ErrorResponse(w http.ResponseWriter, r *http.Request, apiError error) error {
	return errorResponseFor(User, w, r, apiError)
}

// ErrorResponseForServer Marshals error for server requester
func ErrorResponseForServer(w http.ResponseWriter, r *http.Request, apiError error) error {
	return errorResponseFor(Server, w, r, apiError)
}

func errorResponseFor(requesterType Type, w http.ResponseWriter, r *http.Request, apiError error) error {
	var outErr *Error
	var code int
	var ok bool

	// Skip error response if the context is cancelled.
	// This will typically happen when a HTTP request is cancelled by the caller.
	if errors.Is(apiError, context.Canceled) {
		return nil
	}

	err := errors.Cause(apiError)
	if outErr, ok = err.(*Error); !ok {
		switch {
		case errors.Is(apiError, context.DeadlineExceeded):
			outErr = CoverAllError(apiError, Server)
		default:
			outErr = CoverAllError(apiError, requesterType)
		}
	}

	switch e := apiError.(type) {
	case *url.Error:
		// Reflect any underlying network error
		return writeErrorWithCode(w, r, http.StatusInternalServerError, outErr)
	case *k8serrors.StatusError:
		return writeErrorWithCode(w, r, int(e.ErrStatus.Code), outErr)

	default:
		switch outErr.Type {
		case Missing:
			code = http.StatusNotFound
		case User:
			code = http.StatusBadRequest
		case Server:
			code = http.StatusInternalServerError
		case Forbidden:
			code = http.StatusForbidden
		default:
			code = http.StatusInternalServerError
		}
		return writeErrorWithCode(w, r, code, outErr)
	}
}

func writeErrorWithCode(w http.ResponseWriter, r *http.Request, code int, apiError *Error) error {
	// An Accept header with "application/json" is sent by clients
	// understanding how to decode JSON errors. Older clients don't
	// send an Accept header, so we just give them the error text.
	if len(r.Header.Get("Accept")) > 0 {
		contentType, _, err := contenttype.GetAcceptableMediaType(r, supportedMediaTypes)
		if errors.Is(err, contenttype.ErrNoAcceptableTypeFound) {
			w.WriteHeader(http.StatusNotAcceptable)
			return nil
		}
		switch contentType.MIME() {
		case "application/json":
			return writeErrorJSON(w, code, apiError)
		case "text/plain":
			return writeErrorTextPlain(w, code, apiError)
		}
	}
	return writeErrorTextPlain(w, code, apiError)
}

func writeErrorTextPlain(w http.ResponseWriter, code int, apiError *Error) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)

	if apiError.Err != nil {
		_, _ = fmt.Fprintln(w, apiError.Err.Error())
	}

	if len(strings.TrimSpace(apiError.Message)) > 0 {
		_, _ = fmt.Fprintln(w, apiError.Message)
	}

	return nil
}

func writeErrorJSON(w http.ResponseWriter, code int, apiError *Error) error {
	body, encodeErr := json.Marshal(apiError)
	if encodeErr != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		return encodeErr
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, err := w.Write(body)
	return err
}
