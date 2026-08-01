package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/denisakp/ogoune/internal/domain"
	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hostServiceFixture bundles a HostService and the fakes behind it.
type hostServiceFixture struct {
	svc       *HostService
	creds     *HostCredentialService
	credFake  *fake.HostCredentialFake
	metrics   *fake.HostMetricFake
	resources *fake.ResourceFake
	hosts     *fake.HostFake
}

func newHostServiceFixture() *hostServiceFixture {
	hosts := fake.NewHostFake()
	credFake := fake.NewHostCredentialFake()
	creds := NewHostCredentialService(credFake)
	metrics := fake.NewHostMetricFake()
	resources := fake.NewResourceFake()

	svc := NewHostService(hosts, creds, metrics, resources, 45*time.Second)
	return &hostServiceFixture{
		svc:       svc,
		creds:     creds,
		credFake:  credFake,
		metrics:   metrics,
		resources: resources,
		hosts:     hosts,
	}
}

func TestHostService_Register(t *testing.T) {
	ctx := context.Background()
	f := newHostServiceFixture()

	host, raw, prefix, err := f.svc.Register(ctx, "my-host")
	require.NoError(t, err)
	require.NotNil(t, host)
	assert.NotEmpty(t, host.ID)
	assert.Equal(t, "my-host", host.Name)
	require.NotEmpty(t, raw)
	require.NotEmpty(t, prefix)
	assert.True(t, strings.HasPrefix(raw, "ag_live_"))

	// The returned credential authenticates through the creds service.
	cred, err := f.creds.Authenticate(ctx, raw)
	require.NoError(t, err)
	assert.Equal(t, host.ID, cred.HostID)
}

func TestHostService_Register_BlankName(t *testing.T) {
	ctx := context.Background()
	f := newHostServiceFixture()

	_, _, _, err := f.svc.Register(ctx, "   ")
	assert.ErrorIs(t, err, ErrValidationFailed)
}

func TestHostService_Delete_UnlinksMonitorsAndPurges(t *testing.T) {
	ctx := context.Background()
	f := newHostServiceFixture()

	host, _, _, err := f.svc.Register(ctx, "host-to-delete")
	require.NoError(t, err)

	// Seed a monitor and link it to the host.
	res := &domain.Resource{Name: "m", Type: domain.ResourceHTTP}
	_, err = f.resources.Create(ctx, res)
	require.NoError(t, err)

	linked, err := f.svc.LinkMonitor(ctx, res.ID, host.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.HostID)
	assert.Equal(t, host.ID, *linked.HostID)

	// Insert a metric sample for the host.
	require.NoError(t, f.metrics.Insert(ctx, &domain.HostMetricSample{
		HostID:    host.ID,
		SampledAt: time.Now(),
		CPUPct:    10,
		MemPct:    10,
	}))

	// A credential exists for the host.
	credsBefore, err := f.credFake.ListByHost(ctx, host.ID)
	require.NoError(t, err)
	require.NotEmpty(t, credsBefore)

	// Delete the host.
	require.NoError(t, f.svc.Delete(ctx, host.ID))

	// The monitor still exists but is unlinked.
	reloaded, err := f.resources.FindByID(ctx, res.ID)
	require.NoError(t, err)
	assert.Nil(t, reloaded.HostID, "monitor host_id should be cleared, monitor intact")

	// The host is gone.
	_, err = f.svc.Get(ctx, host.ID)
	assert.ErrorIs(t, err, ErrHostNotFound)

	// Metrics purged.
	samples, err := f.metrics.ListInRange(ctx, host.ID, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, samples)

	// Credentials gone.
	credsAfter, err := f.credFake.ListByHost(ctx, host.ID)
	require.NoError(t, err)
	assert.Empty(t, credsAfter)
}

func TestHostService_LinkMonitor_Errors(t *testing.T) {
	ctx := context.Background()
	f := newHostServiceFixture()

	// Unknown host.
	res := &domain.Resource{Name: "m", Type: domain.ResourceHTTP}
	_, err := f.resources.Create(ctx, res)
	require.NoError(t, err)
	_, err = f.svc.LinkMonitor(ctx, res.ID, "no-such-host")
	assert.ErrorIs(t, err, ErrHostNotFound)

	// Known host, unknown monitor.
	host, _, _, err := f.svc.Register(ctx, "host")
	require.NoError(t, err)
	_, err = f.svc.LinkMonitor(ctx, "no-such-monitor", host.ID)
	assert.ErrorIs(t, err, ErrResourceNotFound)
}

func TestHostService_Get_Unknown(t *testing.T) {
	ctx := context.Background()
	f := newHostServiceFixture()

	_, err := f.svc.Get(ctx, "no-such-host")
	assert.ErrorIs(t, err, ErrHostNotFound)
}
