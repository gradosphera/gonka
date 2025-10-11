package apiconfig_test

import (
	"context"
	"decentralized-api/apiconfig"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedsAtomicAdvance(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	dbPath := filepath.Join(tmp, "test.db")

	// minimal static yaml
	require.NoError(t, os.WriteFile(cfgPath, []byte("api:\n  port: 8080\n"), 0644))

	mgr, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, "")
	require.NoError(t, err)

	// Seed initial values
	require.NoError(t, mgr.SetCurrentSeed(apiconfig.SeedInfo{Seed: 1, EpochIndex: 1, Signature: "a"}))
	require.NoError(t, mgr.SetUpcomingSeed(apiconfig.SeedInfo{Seed: 2, EpochIndex: 2, Signature: "b"}))
	require.NoError(t, mgr.FlushNow(context.Background()))

	// Advance: current -> previous, upcoming -> current
	mgr.AdvanceCurrentSeed()
	require.NoError(t, mgr.FlushNow(context.Background()))

	// Read back from DB using getters (hydrated state kept in memory, but we trust flush for DB write)
	ctx := context.Background()
	sPrev, ok, err := apiconfig.GetActiveSeed(ctx, mgr.SqlDb().GetDb(), "previous")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(1), sPrev.Seed)

	sCur, ok, err := apiconfig.GetActiveSeed(ctx, mgr.SqlDb().GetDb(), "current")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(2), sCur.Seed)
}
