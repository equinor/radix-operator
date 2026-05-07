package utils

import (
	"net/http"
	"time"

	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/equinor/radix-operator/api-server/api/metrics"
	"github.com/equinor/radix-operator/api-server/api/middleware/auth"
	"github.com/equinor/radix-operator/api-server/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RadixMiddleware The middleware between router and radix handler functions
type RadixMiddleware struct {
	kubeUtil     KubeUtil
	path         string
	method       string
	allowNoAuth  bool
	kubeApiQPS   float32
	kubeApiBurst int
	next         models.RadixHandlerFunc
}

// NewRadixMiddleware Constructor for radix middleware
func NewRadixMiddleware(kubeUtil KubeUtil, path, method string, allowUnauthenticatedUsers bool, kubeApiQPS float32, kubeApiBurst int, next models.RadixHandlerFunc) *RadixMiddleware {
	handler := &RadixMiddleware{
		kubeUtil,
		path,
		method,
		allowUnauthenticatedUsers,
		kubeApiQPS,
		kubeApiBurst,
		next,
	}

	return handler
}

// Handle Wraps radix handler methods
func (handler *RadixMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	defer func() {
		httpDuration := time.Since(start)
		metrics.AddRequestDuration(handler.path, handler.method, httpDuration)
	}()

	switch {
	case handler.allowNoAuth:
		handler.handleAnonymous(w, r)
	default:
		handler.handleAuthorization(w, r)
	}
}

func (handler *RadixMiddleware) handleAuthorization(w http.ResponseWriter, r *http.Request) {
	logger := log.Ctx(r.Context())
	token := auth.CtxTokenPrincipal(r.Context()).Token()
	impersonation := auth.CtxImpersonation(r.Context())

	restOptions := handler.getRestClientOptions()
	inClusterClient, inClusterRadixClient, inClusterKedaClient, inClusterSecretProviderClient, inClusterTektonClient, inClusterCertManagerClient := handler.kubeUtil.GetServerKubernetesClient(restOptions...)
	outClusterClient, outClusterRadixClient, outClusterKedaClient, outClusterSecretProviderClient, outClusterTektonClient, outClusterCertManagerClient := handler.kubeUtil.GetUserKubernetesClient(token, impersonation, restOptions...)

	accounts := models.NewAccounts(inClusterClient, inClusterRadixClient, inClusterKedaClient, inClusterSecretProviderClient, inClusterTektonClient, inClusterCertManagerClient, outClusterClient, outClusterRadixClient, outClusterKedaClient, outClusterSecretProviderClient, outClusterTektonClient, outClusterCertManagerClient)

	// Check if registration of application exists for application-specific requests
	if appName, exists := mux.Vars(r)["appName"]; exists {
		if _, err := accounts.UserAccount.RadixClient.RadixV1().RadixRegistrations().Get(r.Context(), appName, metav1.GetOptions{}); err != nil {
			logger.Warn().Err(err).Msg("authorization error")
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}
	}

	handler.next(accounts, w, r)
}

func (handler *RadixMiddleware) handleAnonymous(w http.ResponseWriter, r *http.Request) {
	restOptions := handler.getRestClientOptions()
	inClusterClient, inClusterRadixClient, inClusterKedaClient, inClusterSecretProviderClient, inClusterTektonClient, inClusterCertManagerClient := handler.kubeUtil.GetServerKubernetesClient(restOptions...)

	sa := models.NewServiceAccount(inClusterClient, inClusterRadixClient, inClusterKedaClient, inClusterSecretProviderClient, inClusterTektonClient, inClusterCertManagerClient)
	accounts := models.Accounts{ServiceAccount: sa}

	handler.next(accounts, w, r)
}

func (handler *RadixMiddleware) getRestClientOptions() []RestClientConfigOption {
	var options []RestClientConfigOption

	if handler.kubeApiQPS > 0.0 {
		options = append(options, WithQPS(handler.kubeApiQPS))
	}

	if handler.kubeApiBurst > 0 {
		options = append(options, WithBurst(handler.kubeApiBurst))
	}

	return options
}
