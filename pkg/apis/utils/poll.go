package utils

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// PollUntilRESTClientSuccessfulConnection tries a GET request to root with the RESTClient returned by clientFactory.
// clientFactory will be invoked on `interval` for each connection attempt until the request returns no error,
// the error is not `net/http: TLS handshake timeout` or `ctx` is cancelled or hits a deadline.
// Polling will terminate after `duration` defined in timeout.
func PollUntilRESTClientSuccessfulConnection[T interface{ RESTClient() rest.Interface }](ctx context.Context, timeout time.Duration, interval time.Duration, clientFactory func() (T, error)) (T, error) {
	var client T
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
		if err := c.RESTClient().Get().Do(timeoutCtx).Error(); err != nil && isTransientConnectionError(err) {
			log.Ctx(ctx).Info().Err(err).Msg("Transient error when connecting. Retrying")
			return false, nil
		}
		client = c
		return true, nil
	})
	return client, err
}
