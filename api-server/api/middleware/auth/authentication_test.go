package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	controllertest "github.com/equinor/radix-operator/api-server/api/test"
	authnmock "github.com/equinor/radix-operator/api-server/api/utils/token/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestAuthenticatedRequest(t *testing.T) {
	validator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	testPrincipal := controllertest.NewTestPrincipal(true)
	validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(testPrincipal, nil)

	handler := auth.NewAuthenticationMiddleware(validator)
	rw := httptest.NewRecorder()

	var reqCtx context.Context
	nextFn := func(w http.ResponseWriter, r *http.Request) {
		reqCtx = r.Context()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}

	req, _ := http.NewRequest("POST", "/api/anyendpoint", nil)
	req.Header.Add("Authorization", "Bearer "+testPrincipal.Token())
	handler.ServeHTTP(rw, req, nextFn)

	reqPrincipal := auth.CtxTokenPrincipal(reqCtx)
	assert.Same(t, testPrincipal, reqPrincipal)
	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "hello world", rw.Body.String())
}

func TestAnonumousRequest(t *testing.T) {
	validator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	testPrincipal := controllertest.NewTestPrincipal(true)
	validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(0).Return(testPrincipal, nil)

	handler := auth.NewAuthenticationMiddleware(validator)
	rw := httptest.NewRecorder()

	req, _ := http.NewRequest("POST", "/api/anyendpoint", nil)
	handler.ServeHTTP(rw, req, newNullMiddleware())

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "hello world", rw.Body.String())
}

func TestInvalidHeader(t *testing.T) {
	validator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	testPrincipal := controllertest.NewTestPrincipal(true)
	validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(0).Return(testPrincipal, nil)

	handler := auth.NewAuthenticationMiddleware(validator)
	rw := httptest.NewRecorder()

	req, _ := http.NewRequest("POST", "/api/anyendpoint", nil)
	req.Header.Add("Authorization", "Bearerhello-world")

	handler.ServeHTTP(rw, req, newNullMiddleware())
	assert.Equal(t, http.StatusBadRequest, rw.Code)
	assert.Contains(t, rw.Body.String(), "Authentication header is invalid")
}

func TestInvalidToken(t *testing.T) {
	validator := authnmock.NewMockValidatorInterface(gomock.NewController(t))
	testPrincipal := controllertest.NewTestPrincipal(true)
	validator.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(1).Return(testPrincipal, errors.New("some error"))

	handler := auth.NewAuthenticationMiddleware(validator)
	rw := httptest.NewRecorder()

	req, _ := http.NewRequest("POST", "/api/anyendpoint", nil)
	req.Header.Add("Authorization", "Bearer hello-world")

	handler.ServeHTTP(rw, req, newNullMiddleware())
	assert.Equal(t, http.StatusBadRequest, rw.Code)
	assert.Contains(t, rw.Body.String(), "some error")
}

func newNullMiddleware() func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("hello world"))
	}
}
