package apiconfig_test

import (
	"context"
	"decentralized-api/apiconfig"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(contents), 0644))
	return p
}

// Scenario 2: Migration + node-config merge on first run (merged_node_config=false)
func TestMigrationAndNodeConfigMerge_FirstRun(t *testing.T) {
	tmp := t.TempDir()

	yaml := `api:
  port: 8080
current_height: 5
nodes:
  - host: http://yaml-node:8080/
    models:
      modelY: {args: []}
    id: yaml-node
    max_concurrent: 3
merged_node_config: false
`
	cfgPath := writeTempFile(t, tmp, "config.yaml", yaml)

	nodeJson := `[{"host":"http://json-node:8080/","models":{"modelZ":{"args":[]}},"id":"json-node","max_concurrent":2,"hardware":[]}]`
	nodePath := writeTempFile(t, tmp, "node-config.json", nodeJson)
	dbPath := filepath.Join(tmp, "test.db")
	_ = os.Remove(dbPath)

	mgr, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, nodePath)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, mgr.FlushNow(ctx))

	// KV flags set
	var dummy bool
	ok, err := apiconfig.KVGetJSON(ctx, mgr.SqlDb().GetDb(), "config_migrated", &dummy)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = apiconfig.KVGetJSON(ctx, mgr.SqlDb().GetDb(), "node_config_merged", &dummy)
	require.NoError(t, err)
	require.True(t, ok)

	nodes, err := apiconfig.ReadNodes(ctx, mgr.SqlDb().GetDb())
	require.NoError(t, err)
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.Id)
	}
	b, _ := json.Marshal(ids)
	// Because merged_node_config was false, JSON must win on first run
	require.Contains(t, ids, "json-node", string(b))
}

// Scenario 3: First run without node-config.json (merged_node_config=false)
func TestFirstRun_NoNodeConfig_UsesYamlNodesAndStripsDynamicFromYaml(t *testing.T) {
	tmp := t.TempDir()
	yaml := `api:
  port: 8080
current_height: 7
nodes:
  - host: http://yaml-node:8080/
    models:
      modelY: {args: []}
    id: yaml-node
    max_concurrent: 3
merged_node_config: false
`
	cfgPath := writeTempFile(t, tmp, "config.yaml", yaml)
	// No node-config.json
	dbPath := filepath.Join(tmp, "test.db")
	_ = os.Remove(dbPath)

	mgr, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, "")
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, mgr.FlushNow(ctx))

	// DB has yaml nodes
	nodes, err := apiconfig.ReadNodes(ctx, mgr.SqlDb().GetDb())
	require.NoError(t, err)
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.Id)
	}
	require.Contains(t, ids, "yaml-node")

	// After Write(), static-only YAML should be persisted; dynamic fields removed
	// Call Write() explicitly and re-read file
	require.NoError(t, mgr.Write())
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	s := string(data)
	require.NotContains(t, s, "current_height:")
}

// Scenario 4: Relaunch after migration; ignore dynamic YAML and skip migration
func TestRelaunchAfterMigration_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	yaml1 := `api:\n  port: 8080\ncurrent_height: 10\nnodes:\n  - host: http://yaml-node:8080/\n    models:\n      modelY: {args: []}\n    id: yaml-node\n    max_concurrent: 3\nmerged_node_config: false\n`
	cfgPath := writeTempFile(t, tmp, "config.yaml", yaml1)

	// First run with JSON nodes imported
	nodeJson := `[{"host":"http://json-node:8080/","models":{"modelZ":{"args":[]}},"id":"json-node","max_concurrent":2,"hardware":[]}]`
	nodePath := writeTempFile(t, tmp, "node-config.json", nodeJson)
	_ = os.Remove(dbPath)
	mgr, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, nodePath)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, mgr.FlushNow(ctx))

	// Second run with conflicting YAML dynamic (try to re-introduce nonsense)
	yaml2 := `api:
  port: 8080
current_height: 999
merged_node_config: true
nodes:
  - host: http://yaml-node2:8080/
    models:
      modelY: {args: []}
    id: yaml-node2
    max_concurrent: 3
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml2), 0644))
	mgr2, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, nodePath)
	require.NoError(t, err)
	require.NoError(t, mgr2.FlushNow(ctx))

	nodes, err := apiconfig.ReadNodes(ctx, mgr2.SqlDb().GetDb())
	require.NoError(t, err)
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.Id)
	}
	b, _ := json.Marshal(ids)
	// Must still be json-node from DB, not yaml-node2
	require.Contains(t, ids, "json-node", string(b))
	require.NotContains(t, ids, "yaml-node2", string(b))
}
