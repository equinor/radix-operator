package utils

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PollUntilClientSuccessfulConnection tries a GET request to root with the RESTClient returned by clientFactory.
// clientFactory will be invoked on `interval` for each connection attempt until the request returns no error,
// the error is not `net/http: TLS handshake timeout` or `ctx` is cancelled or hits a deadline.
// Polling will terminate after `duration` defined in timeout.
func PollUntilClientSuccessfulConnection(ctx context.Context, timeout time.Duration, interval time.Duration, clientFactory func() (client.WithWatch, error)) (client.WithWatch, error) {
	var connectedClient client.WithWatch
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := wait.PollUntilContextCancel(timeoutCtx, interval, true, func(ctx context.Context) (done bool, err error) {
		c, err := clientFactory()
		if err != nil {
			return false, err
		}

		isTransientConnectionError := func(err error) bool {
			// TODO: Should we check for other connection errors that are transient, e.g. net.DNSError?
			return err.Error() == "net/http: TLS handshake timeout"
		}

		// Retry if error transient, e.g. TLS handshake timeout
		ns := corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: os.Getenv("POD_NAMESPACE")}}
		if err := c.Get(timeoutCtx, client.ObjectKeyFromObject(&ns), &ns); err != nil && isTransientConnectionError(err) {
			log.Ctx(ctx).Info().Err(err).Msg("Transient error when connecting. Retrying")
			return false, nil
		}
		connectedClient = c
		return true, nil
	})
	return connectedClient, err
}
