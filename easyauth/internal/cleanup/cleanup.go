// Package cleanup deletes Dex clients owned by EasyAuth that no longer belong to an application
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/dexidp/dex/api/v2"
	"github.com/rs/zerolog/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/equinor/radix-operator/easyauth/internal/dex"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// DexClientCleaner periodically deletes Dex clients with the EasyAuth prefix that don't belong to a
// RadixApplication with the annotation. See "Cleanup loop" in easyauth/README.md.
type DexClientCleaner struct {
	reader client.Reader
	// Waits for the cache used by reader to sync before the first run. Set by SetupWithManager.
	waitForCacheSync func(ctx context.Context) bool
	dex              api.DexClient
	annotation       string
	interval         time.Duration
}

// NewDexClientCleaner returns a cleaner that deletes Dex clients every interval, unless they belong to a
// RadixApplication with annotation. reader reads RadixApplications and RadixRegistrations, normally from the
// manager cache.
func NewDexClientCleaner(reader client.Reader, dex api.DexClient, annotation string, interval time.Duration) *DexClientCleaner {
	return &DexClientCleaner{
		reader:           reader,
		waitForCacheSync: func(context.Context) bool { return true },
		dex:              dex,
		annotation:       annotation,
		interval:         interval,
	}
}

// SetupWithManager adds the cleaner to the Manager. It waits for the Manager cache to sync before the first run.
func (c *DexClientCleaner) SetupWithManager(mgr ctrl.Manager) error {
	c.waitForCacheSync = mgr.GetCache().WaitForCacheSync
	return mgr.Add(c)
}

// Start runs the cleanup every interval until ctx is cancelled. Errors are logged, and the cleanup is retried on the next run.
func (c *DexClientCleaner) Start(ctx context.Context) error {
	logger := log.Ctx(ctx).With().Str("runnable", "dex-client-cleanup").Logger()
	ctx = logger.WithContext(ctx)

	if !c.waitForCacheSync(ctx) {
		return errors.New("cache did not sync")
	}

	for {
		if err := c.Cleanup(ctx); err != nil {
			logger.Error().Err(err).Msg("Failed to clean up Dex clients")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.Tick(c.interval):
		}
	}
}

// NeedLeaderElection makes the cleanup only run on the leader
func (c *DexClientCleaner) NeedLeaderElection() bool {
	return true
}

// Cleanup deletes Dex clients with the EasyAuth prefix that don't belong to a RadixApplication with the annotation.
// Nothing is deleted if Dex or the Kubernetes API returns an error.
func (c *DexClientCleaner) Cleanup(ctx context.Context) error {
	// Clients must be listed before RadixApplications. A client is only created after its RadixApplication exists,
	// so a client created after this point is never deleted by mistake.
	resp, err := c.dex.ListClients(ctx, &api.ListClientReq{})
	if err != nil {
		return fmt.Errorf("failed to list Dex clients: %w", err)
	}

	inUse, err := c.clientIDsInUse(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, dexClient := range resp.GetClients() {
		id := dexClient.GetId()
		if !strings.HasPrefix(id, dex.ClientIDPrefix) || inUse[id] {
			continue
		}
		if _, err := c.dex.DeleteClient(ctx, &api.DeleteClientReq{Id: id}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete Dex client %s: %w", id, err))
			continue
		}
		log.Ctx(ctx).Info().Str("clientId", id).Str("name", dexClient.GetName()).Msg("Deleted Dex client")
	}
	return errors.Join(errs...)
}

// clientIDsInUse returns the client IDs of all RadixApplications with the annotation
func (c *DexClientCleaner) clientIDsInUse(ctx context.Context) (map[string]bool, error) {
	raList := &radixv1.RadixApplicationList{}
	if err := c.reader.List(ctx, raList); err != nil {
		return nil, fmt.Errorf("failed to list RadixApplications: %w", err)
	}
	rrList := &radixv1.RadixRegistrationList{}
	if err := c.reader.List(ctx, rrList); err != nil {
		return nil, fmt.Errorf("failed to list RadixRegistrations: %w", err)
	}

	appIDs := make(map[string]radixv1.ULID, len(rrList.Items))
	for _, rr := range rrList.Items {
		appIDs[rr.Name] = rr.Spec.AppID
	}

	inUse := map[string]bool{}
	for _, ra := range raList.Items {
		if _, ok := ra.Annotations[c.annotation]; !ok {
			continue
		}
		if appID, ok := appIDs[ra.Name]; ok && !appID.IsZero() {
			inUse[dex.ClientID(appID.String())] = true
		}
	}
	return inUse, nil
}

var _ manager.LeaderElectionRunnable = &DexClientCleaner{}
var _ manager.Runnable = &DexClientCleaner{}
