package apiconfig_test

import (
	"bytes"
	"context"
	"decentralized-api/apiconfig"
	"decentralized-api/logging"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
	testManager := &apiconfig.ConfigManager{
		KoanProvider: rawbytes.Provider([]byte(testYaml)),
	}
	err := testManager.Load()
	require.NoError(t, err)
	require.Equal(t, 8080, testManager.GetApiConfig().Port)
	require.Equal(t, "http://join1-node:26657", testManager.GetChainNodeConfig().Url)
	require.Equal(t, "join1", testManager.GetChainNodeConfig().SignerKeyName)
	require.Equal(t, "test", testManager.GetChainNodeConfig().KeyringBackend)
	require.Equal(t, "/root/.inference", testManager.GetChainNodeConfig().KeyringDir)
}

func TestNodeVersion(t *testing.T) {
	testManager := &apiconfig.ConfigManager{
		KoanProvider:   rawbytes.Provider([]byte(testYaml)),
		WriterProvider: &CaptureWriterProvider{},
	}
	err := testManager.Load()
	require.NoError(t, err)

	// Test that default version is returned correctly
	require.Equal(t, testManager.GetCurrentNodeVersion(), "v3.0.8")
}

func TestConfigLoadEnvOverride(t *testing.T) {
	testManager := &apiconfig.ConfigManager{
		KoanProvider: rawbytes.Provider([]byte(testYaml)),
	}

	os.Setenv("DAPI_API__PORT", "9000")
	os.Setenv("KEY_NAME", "join2")
	os.Setenv("DAPI_CHAIN_NODE__URL", "http://join1-node:26658")
	os.Setenv("DAPI_API__POC_CALLBACK_URL", "http://callback")
	os.Setenv("DAPI_API__PUBLIC_URL", "http://public")
	err := testManager.Load()
	require.NoError(t, err)
	require.Equal(t, 9000, testManager.GetApiConfig().Port)
	require.Equal(t, "http://join1-node:26658", testManager.GetChainNodeConfig().Url)
	require.Equal(t, "join2", testManager.GetChainNodeConfig().SignerKeyName)
	require.Equal(t, "http://callback", testManager.GetApiConfig().PoCCallbackUrl)
	require.Equal(t, "http://public", testManager.GetApiConfig().PublicUrl)
	require.Equal(t, "test", testManager.GetChainNodeConfig().KeyringBackend)
	require.Equal(t, "/root/.inference", testManager.GetChainNodeConfig().KeyringDir)

}

type CaptureWriterProvider struct {
	CapturedData string
}

func (c *CaptureWriterProvider) Write(data []byte) (int, error) {
	c.CapturedData += string(data)
	return len(data), nil
}

func (c *CaptureWriterProvider) Close() error {
	return nil
}

func (c *CaptureWriterProvider) GetWriter() apiconfig.WriteCloser {
	return c
}

func TestConfigRoundTrip(t *testing.T) {
	os.Unsetenv("DAPI_API__PORT")
	os.Unsetenv("KEY_NAME")
	os.Unsetenv("DAPI_CHAIN_NODE__URL")
	os.Unsetenv("DAPI_API__POC_CALLBACK_URL")
	os.Unsetenv("DAPI_API__PUBLIC_URL")
	writeCapture := &CaptureWriterProvider{}
	testManager := &apiconfig.ConfigManager{
		KoanProvider:   rawbytes.Provider([]byte(testYaml)),
		WriterProvider: writeCapture,
	}
	err := testManager.Load()
	require.NoError(t, err)

	// Test can write config successfully
	err = testManager.Write()
	require.NoError(t, err)

	t.Log("\n")
	t.Log(writeCapture.CapturedData)
	testManager2 := &apiconfig.ConfigManager{
		KoanProvider:   rawbytes.Provider([]byte(writeCapture.CapturedData)),
		WriterProvider: writeCapture,
	}
	err = testManager2.Load()

	testManager2.SetHeight(50)
	require.NoError(t, err)
	require.Equal(t, 8080, testManager2.GetApiConfig().Port)
	require.Equal(t, "http://join1-node:26657", testManager2.GetChainNodeConfig().Url)
	require.Equal(t, "join1", testManager2.GetChainNodeConfig().SignerKeyName)
	require.Equal(t, "test", testManager2.GetChainNodeConfig().KeyringBackend)
	require.Equal(t, "/root/.inference", testManager2.GetChainNodeConfig().KeyringDir)
	// After Write() we persist only static fields; dynamic fields like version are not in YAML
	require.Equal(t, "", testManager2.GetCurrentNodeVersion())
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// Scenario 1: YAML contains dynamic state and merged flag; DB empty; node-config.json present but must be ignored.
func TestLoadDefaultConfigManager_Migration_Idempotent_And_NodeConfigSkipped(t *testing.T) {
	t.Setenv("DAPI_API__PORT", "")
	t.Setenv("KEY_NAME", "")
	t.Setenv("DAPI_CHAIN_NODE__URL", "")
	t.Setenv("DAPI_API__POC_CALLBACK_URL", "")
	t.Setenv("DAPI_API__PUBLIC_URL", "")

	tmp := t.TempDir()

	yaml := `api:
  port: 8080
chain_node:
  url: http://join1-node:26657
  signer_key_name: join1
  account_public_key: ""
  keyring_backend: test
  keyring_dir: /root/.inference
current_height: 393
current_seed:
  seed: 3898730504561900192
  epoch_index: 380
  signature: abc
previous_seed:
  seed: 1370553182438852893
  epoch_index: 370
  signature: def
upcoming_seed:
  seed: 254929314898674592
  epoch_index: 390
  signature: ghi
nodes:
  - host: http://yaml-node:8080/
    models:
      modelA: {args: []}
    id: yaml-node-1
    max_concurrent: 5
merged_node_config: true
current_node_version: "v3.0.8"
`
	cfgPath := writeTemp(t, tmp, "config.yaml", yaml)

	nodeJson := `[
  {
    "host": "http://json-node:8080/",
    "inference_segment": "",
    "inference_port": 0,
    "poc_segment": "",
    "poc_port": 0,
    "models": {"modelB": {"args": []}},
    "id": "json-node-1",
    "max_concurrent": 10,
    "hardware": []
  }
]`
	nodePath := writeTemp(t, tmp, "node-config.json", nodeJson)
	dbPath := filepath.Join(tmp, "test.db")
	// Ensure no leftover file exists
	_ = os.Remove(dbPath)

	// First load -> migration and hydrate
	mgr, err := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, nodePath)
	require.NoError(t, err)
	ctx := context.Background()
	ok, err := apiconfig.KVGetJSON(ctx, mgr.SqlDb().GetDb(), "config_migrated", new(bool))
	require.NoError(t, err)
	require.True(t, ok)

	// Ensure nodes persisted reflect YAML, not JSON (merged_node_config=true)
	require.NoError(t, mgr.FlushNow(ctx))
	nodes, err := apiconfig.ReadNodes(ctx, mgr.SqlDb().GetDb())
	require.NoError(t, err)
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.Id)
	}
	b, _ := json.Marshal(ids)
	require.NotContains(t, ids, "json-node-1", string(b))
	require.Contains(t, ids, "yaml-node-1", string(b))

	// Second load -> idempotent
	mgr2, err2 := apiconfig.LoadConfigManagerWithPaths(cfgPath, dbPath, nodePath)
	require.NoError(t, err2)
	require.NoError(t, mgr2.FlushNow(ctx))
	nodes2, err2 := apiconfig.ReadNodes(ctx, mgr2.SqlDb().GetDb())
	require.NoError(t, err2)
	ids2 := make([]string, 0, len(nodes2))
	for _, n := range nodes2 {
		ids2 = append(ids2, n.Id)
	}
	require.Equal(t, ids, ids2)

	// Cleanup test DB explicitly
	require.NoError(t, mgr2.FlushNow(ctx))
	_ = os.Remove(dbPath)
}

func loadManager(t *testing.T) error {
	testManager := &apiconfig.ConfigManager{
		KoanProvider: rawbytes.Provider([]byte(testYaml)),
	}
	os.Setenv("DAPI_API__PORT", "9000")
	os.Setenv("DAPI_CHAIN_NODE__URL", "http://join1-node:26658")
	os.Setenv("KEY_NAME", "join2")
	defer func() {
		// Clean up environment variables
		os.Unsetenv("DAPI_API__PORT")
		os.Unsetenv("DAPI_CHAIN_NODE__URL")
		os.Unsetenv("KEY_NAME")
	}()

	if err := testManager.Load(); err != nil {
		return err
	}
	return nil
}

// We cannot write anything to stdout when loading config or we break cosmovisor!
func TestNoLoggingToStdout(t *testing.T) {
	// Save the original stdout
	originalStdout := os.Stdout
	defer func() { os.Stdout = originalStdout }() // Restore it after the test

	// Create a pipe to capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Buffer to capture log output
	var buf bytes.Buffer

	// Load config with overrides
	_, noLoggerErr := logging.WithNoopLogger(func() (interface{}, error) {
		err := loadManager(t)
		return nil, err
	})
	require.NoError(t, noLoggerErr)

	// Close the pipe and reset stdout
	_ = w.Close()
	os.Stdout = originalStdout

	// Read captured output
	_, _ = buf.ReadFrom(r)

	// Fail if anything is written to stdout
	if buf.Len() > 0 {
		t.Errorf("Unexpected logging to stdout: %q", buf.String())
	}
}

var testYaml = `
api:
    port: 8080
chain_node:
    url: http://join1-node:26657
    signer_key_name: join1
    account_public_key: ""
    keyring_backend: test
    keyring_dir: /root/.inference
current_height: 393
current_seed:
    seed: 3898730504561900192
    height: 380
    signature: 815794b7bbb414900a84c8a543ffc96a3ebb5fbbd0175648eaf5f60897b786df5a0be5bc6047ee2ac3c8c2444510fcb9a1f565a6359927226f619dd534035bb7
nodes:
    - url: http://34.171.235.205:8080/
      models:
        - Qwen/Qwen2.5-7B-Instruct: {}
      id: node1
      max_concurrent: 500
previous_seed:
    seed: 1370553182438852893
    height: 370
    signature: 1d1f9fc6f44840af03368ce24e0335834181e42a9a45c81d7a17e14866729fa81a08c14d3397e00d4b16da3ab708e284650f8b14b33b318820ae0524b6ead6db
upcoming_seed:
    seed: 254929314898674592
    height: 390
    signature: 75296c164d43e5570c44c88176c7988e7d52d3e44be6c43e8e6c8f07327279510092f429addc401665d6ed128725f2181a95c7aba66c89ea77209c55ef2ce342
upgrade_plan:
    name: ""
    height: 0
    binaries: {}
current_node_version: "v3.0.8"
`
