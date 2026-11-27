package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const webhookCacheSyncTimeout = 30 * time.Second

// createWhenWebhookCacheIsSynced calls create until the error, if any, no longer contains notSyncedMessage.
// The webhooks validate against an informer cache, so resources created just before, e.g. a namespace or a
// RadixRegistration, can briefly be missing in the webhook. It returns the error of the last call.
func createWhenWebhookCacheIsSynced(t *testing.T, notSyncedMessage string, create func(ctx context.Context) error) error {
	t.Helper()
	var lastErr error
	_ = wait.PollUntilContextTimeout(t.Context(), 500*time.Millisecond, webhookCacheSyncTimeout, true, func(ctx context.Context) (bool, error) {
		lastErr = create(ctx)
		return lastErr == nil || !strings.Contains(lastErr.Error(), notSyncedMessage), nil
	})
	return lastErr
}
