package service

import (
	"context"
	"strings"
	"testing"

	"github.com/denisakp/ogoune/internal/repository/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostCredentialService_IssueThenAuthenticate(t *testing.T) {
	ctx := context.Background()
	svc := NewHostCredentialService(fake.NewHostCredentialFake())

	const hostID = "host-1"
	raw, prefix, err := svc.Issue(ctx, hostID)
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	require.NotEmpty(t, prefix)
	assert.True(t, strings.HasPrefix(raw, "ag_live_"), "raw credential should be ag_live_ formatted")

	cred, err := svc.Authenticate(ctx, raw)
	require.NoError(t, err)
	require.NotNil(t, cred)
	assert.Equal(t, hostID, cred.HostID)
}

func TestHostCredentialService_Authenticate_Rejects(t *testing.T) {
	ctx := context.Background()
	svc := NewHostCredentialService(fake.NewHostCredentialFake())

	// A well-formed but unknown token.
	_, err := svc.Authenticate(ctx, "ag_live_deadbeefdeadbeefdeadbeef")
	assert.ErrorIs(t, err, ErrHostCredentialInvalid)

	// A token not in ag_live_ format.
	_, err = svc.Authenticate(ctx, "garbage-token")
	assert.ErrorIs(t, err, ErrHostCredentialInvalid)

	// Empty token.
	_, err = svc.Authenticate(ctx, "")
	assert.ErrorIs(t, err, ErrHostCredentialInvalid)
}

func TestHostCredentialService_Revoke(t *testing.T) {
	ctx := context.Background()
	svc := NewHostCredentialService(fake.NewHostCredentialFake())

	const hostID = "host-1"
	raw, _, err := svc.Issue(ctx, hostID)
	require.NoError(t, err)

	// Sanity: authenticates before revoke.
	_, err = svc.Authenticate(ctx, raw)
	require.NoError(t, err)

	require.NoError(t, svc.Revoke(ctx, hostID))

	// After revoke the previously-issued raw no longer authenticates.
	_, err = svc.Authenticate(ctx, raw)
	assert.ErrorIs(t, err, ErrHostCredentialInvalid)
}

func TestHostCredentialService_Rotate(t *testing.T) {
	ctx := context.Background()
	svc := NewHostCredentialService(fake.NewHostCredentialFake())

	const hostID = "host-1"
	oldRaw, _, err := svc.Issue(ctx, hostID)
	require.NoError(t, err)

	newRaw, newPrefix, err := svc.Rotate(ctx, hostID)
	require.NoError(t, err)
	require.NotEmpty(t, newRaw)
	require.NotEmpty(t, newPrefix)
	assert.NotEqual(t, oldRaw, newRaw, "rotate should issue a brand new raw credential")

	// New raw works.
	cred, err := svc.Authenticate(ctx, newRaw)
	require.NoError(t, err)
	assert.Equal(t, hostID, cred.HostID)

	// Old raw stops authenticating.
	_, err = svc.Authenticate(ctx, oldRaw)
	assert.ErrorIs(t, err, ErrHostCredentialInvalid)
}
