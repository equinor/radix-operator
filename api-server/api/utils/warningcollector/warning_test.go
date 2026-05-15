package warningcollector_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/utils/warningcollector"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/negroni/v3"
)

func TestCollector(t *testing.T) {
	var warnings []string
	handler := warningcollector.NewKubernetesWarningHandler()

	n := negroni.New()
	n.Use(warningcollector.NewWarningCollectorMiddleware())
	n.UseFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		handler.HandleWarningHeaderWithContext(r.Context(), 299, "test-agent", "This is a test warning")

		warnings = warningcollector.GetWarningCollectionFromContext(r.Context())
		next(w, r)
	})
	n.UseHandler(http.NotFoundHandler())
	n.ServeHTTP(httptest.NewRecorder(), &http.Request{})

	assert.NotEmpty(t, warnings, "Warnings should not be empty")
	assert.Contains(t, warnings, "This is a test warning", "Warning should be collected")
}

func TestEmptyCollector(t *testing.T) {
	var warnings []string
	n := negroni.New()
	n.Use(warningcollector.NewWarningCollectorMiddleware())
	n.UseFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		warnings = warningcollector.GetWarningCollectionFromContext(r.Context())
		next(w, r)
	})
	n.UseHandler(http.NotFoundHandler())
	n.ServeHTTP(httptest.NewRecorder(), &http.Request{})

	assert.Empty(t, warnings, "Warnings should be empty")
}
