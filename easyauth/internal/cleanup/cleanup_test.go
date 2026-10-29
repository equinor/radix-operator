package cleanup

import (
	"context"
	"testing"
	"time"

	api "github.com/dexidp/dex/api/v2"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/equinor/radix-operator/easyauth/internal/dex"
	"github.com/equinor/radix-operator/easyauth/internal/dex/dextest"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/scheme"
)

const annotation = "radix.equinor.com/easyauth"

type app struct {
	name      string
	appID     ulid.ULID
	annotated bool
	noRA      bool
}

func newCleaner(fakeDex *dextest.FakeDex, apps ...app) *DexClientCleaner {
	var objs []client.Object
	for _, a := range apps {
		objs = append(objs, &radixv1.RadixRegistration{Name: a.name, Spec: radixv1.RadixRegistrationSpec{AppID: radixv1.ULID{ULID: a.appID}}})
		if a.noRA {
			continue
		}
		ra := &radixv1.RadixApplication{Name: a.name, Namespace: a.name + "-app"}
		if a.annotated {
			ra.Annotations = map[string]string{annotation: ""}
		}
		objs = append(objs, ra)
	}
	reader := fake.NewClientBuilder().WithScheme(scheme.NewScheme()).WithObjects(objs...).Build()
	return NewDexClientCleaner(reader, fakeDex, annotation, time.Hour)
}

func dexClient(id string) *api.Client {
	return &api.Client{Id: id, Name: "name", Secret: "secret"}
}

func TestCleanup_DeletesOrphanedEasyAuthClients(t *testing.T) {
	inUse, notAnnotated, noRA, recreatedOld, recreatedNew := ulid.Make(), ulid.Make(), ulid.Make(), ulid.Make(), ulid.Make()
	fakeDex := dextest.NewFakeDex(
		dexClient(dex.ClientID(inUse.String())),
		dexClient(dex.ClientID(notAnnotated.String())),
		dexClient(dex.ClientID(noRA.String())),
		// Client of a deleted application whose name was reused by a new application
		dexClient(dex.ClientID(recreatedOld.String())),
		dexClient(dex.ClientID(recreatedNew.String())),
		// Client of a deleted RadixRegistration
		dexClient(dex.ClientID(ulid.Make().String())),
		// Not owned by EasyAuth
		dexClient("radix-web-console"),
	)
	c := newCleaner(fakeDex,
		app{name: "in-use", appID: inUse, annotated: true},
		app{name: "not-annotated", appID: notAnnotated},
		app{name: "no-ra", appID: noRA, noRA: true},
		app{name: "recreated", appID: recreatedNew, annotated: true},
	)

	require.NoError(t, c.Cleanup(context.Background()))

	assert.NotNil(t, fakeDex.Client(dex.ClientID(inUse.String())))
	assert.NotNil(t, fakeDex.Client(dex.ClientID(recreatedNew.String())))
	assert.NotNil(t, fakeDex.Client("radix-web-console"), "clients without the EasyAuth prefix must not be deleted")
	assert.Nil(t, fakeDex.Client(dex.ClientID(notAnnotated.String())))
	assert.Nil(t, fakeDex.Client(dex.ClientID(noRA.String())))
	assert.Nil(t, fakeDex.Client(dex.ClientID(recreatedOld.String())))
	assert.Equal(t, 4, fakeDex.Calls("DeleteClient"))
}

func TestCleanup_DeletesNothingWhenDexFails(t *testing.T) {
	fakeDex := dextest.NewFakeDex(dexClient(dex.ClientID(ulid.Make().String())))
	fakeDex.SetError(status.Error(codes.Unavailable, "connection refused"))
	c := newCleaner(fakeDex)

	assert.Error(t, c.Cleanup(context.Background()))
	assert.Zero(t, fakeDex.Calls("DeleteClient"))
}

func TestCleanup_DeletesNothingWhenKubernetesFails(t *testing.T) {
	fakeDex := dextest.NewFakeDex(dexClient(dex.ClientID(ulid.Make().String())))
	c := newCleaner(fakeDex)
	// The scheme doesn't know the Radix types, so listing RadixApplications fails
	c.reader = fake.NewClientBuilder().Build()

	assert.Error(t, c.Cleanup(context.Background()))
	assert.Zero(t, fakeDex.Calls("DeleteClient"))
}

func TestStart_RunsCleanupUntilCancelled(t *testing.T) {
	orphan := dex.ClientID(ulid.Make().String())
	fakeDex := dextest.NewFakeDex(dexClient(orphan))
	c := newCleaner(fakeDex)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- c.Start(ctx) }()

	require.Eventually(t, func() bool { return fakeDex.Client(orphan) == nil }, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after the context was cancelled")
	}
}

func TestStart_FailsWhenCacheDoesNotSync(t *testing.T) {
	fakeDex := dextest.NewFakeDex()
	c := newCleaner(fakeDex)
	c.waitForCacheSync = func(context.Context) bool { return false }

	assert.Error(t, c.Start(context.Background()))
	assert.Zero(t, fakeDex.Calls("ListClients"))
}

func TestNeedLeaderElection(t *testing.T) {
	assert.True(t, (&DexClientCleaner{}).NeedLeaderElection())
}
