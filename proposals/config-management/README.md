# Configuration Management System Redesign

## Current State Analysis

The decentralized API currently uses a single YAML configuration file that is frequently rewritten, causing file corruption issues. The system performs approximately 10+ different write operations to the same config file:

- **Height updates** (every block) - `ConfigManager.SetHeight()` in `config_manager.go:104`
- **Seed information updates** - `SetCurrentSeed()`, `SetUpcomingSeed()`, `SetPreviousSeed()` in `config_manager.go:165-183`
- **Validation parameters** - `SetValidationParams()` in `config_manager.go:119`
- **Bandwidth parameters** - `SetBandwidthParams()` in `config_manager.go:131`
- **Node version updates** - `AddNodeVersion()` in `config_manager.go:143`
- **Upgrade plans** - `SetUpgradePlan()` in `config_manager.go:94`
- **Node configurations** (via REST API: POST/DELETE `/admin/v1/nodes`) - `syncNodesWithConfig()` in `node_handlers.go:42`
- **Worker key generation** - `CreateWorkerKey()` in `config_manager.go:191`

### Problems with Current Approach

1. **File Corruption Risk**: Frequent rewrites of the entire YAML file with `os.O_TRUNC` flag (`FileWriteCloserProvider.GetWriter()` in `config_manager.go:228`) can cause corruption during concurrent access or system crashes
2. **Performance Issues**: Full file rewrite for single field updates is inefficient - every `writeConfig()` call in `config_manager.go:283` marshals and writes the entire config
3. **Race Conditions**: Despite mutex protection (`config_manager.go:27`), file-level corruption can still occur
4. **Atomic Operation Failures**: No transactional guarantees for configuration updates - if `writer.Write()` fails in `config_manager.go:296`, state is inconsistent
5. **Backup/Recovery Complexity**: Single file corruption affects entire configuration state

## Proposed Solution: Hybrid Configuration Architecture

### Core Principle: Separate Static from Dynamic Data

**Static Configuration (YAML File)**
- Application startup parameters
- **API endpoints and ports** (`ApiConfig` in `config.go:97-107`)
- **Cryptographic keys and credentials** (`MLNodeKeyConfig` in `config.go:120-123`)
- **NATS server configuration** (`NatsServerConfig` in `config.go:21-24`)
- **Chain node connection settings** (`ChainNodeConfig` in `config.go:109-118`)
- Settings that require manual intervention or restart

**Dynamic State (Embedded Database)**
- **Chain height tracking** (`CurrentHeight` in `config.go:11`, updated via `SetHeight()`)
- **Seed information** (`UpcomingSeed`, `CurrentSeed`, `PreviousSeed` in `config.go:8-10`)
- **Network parameters cache** (`ValidationParamsCache`, `BandwidthParamsCache` in `config.go:17-18`)
- **Node version stack** (`NodeVersions` in `config.go:14`, managed via `NodeVersionStack` in `config.go:26-78`)
- **Upgrade plans** (`UpgradePlan` in `config.go:12`)
- **Inference node registry** (`Nodes []InferenceNodeConfig` in `config.go:5`, created/updated/deleted via REST API endpoints in `node_handlers.go`)
- Runtime statistics and metrics

### Architecture Design

```
┌─────────────────────┐    ┌──────────────────────┐
│   Static Config     │    │   Dynamic State DB   │
│   (config.yaml)     │    │   (embedded SQLite)  │
├─────────────────────┤    ├──────────────────────┤
│ • API Config        │    │ • Current Height     │
│ • Chain Node Config │    │ • Seed Information   │
│ • ML Node Keys      │    │ • Validation Params  │
│ • NATS Config       │    │ • Bandwidth Params   │
│ • Static Settings   │    │ • Node Versions      │
│                     │    │ • Upgrade Plans      │
│                     │    │ • Node Registry      │
│                     │    │   (REST API managed) │
└─────────────────────┘    └──────────────────────┘
           │                           │
           └─────────┬─────────────────┘
                     │
           ┌─────────▼─────────┐
           │  ConfigManager    │
           │  - Load static    │
           │  - Init DB        │
           │  - Provide API    │
           │  - Handle updates │
           └───────────────────┘
```

### Database Schema

```sql
-- Chain state tracking
CREATE TABLE chain_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Seed information with history
CREATE TABLE seed_info (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL, -- 'current', 'previous', 'upcoming'
    seed INTEGER NOT NULL,
    epoch_index INTEGER NOT NULL,
    signature TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

-- Network parameters cache
CREATE TABLE network_params (
    param_type TEXT PRIMARY KEY, -- 'validation', 'bandwidth'
    param_data TEXT NOT NULL, -- JSON blob
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node version stack
CREATE TABLE node_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    height INTEGER NOT NULL,
    version TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(height, version)
);

-- Upgrade plans
CREATE TABLE upgrade_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    height INTEGER NOT NULL,
    binaries TEXT NOT NULL, -- JSON blob
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Inference node registry (managed via REST API)
CREATE TABLE inference_nodes (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    inference_segment TEXT NOT NULL,
    inference_port INTEGER NOT NULL,
    poc_segment TEXT NOT NULL,
    poc_port INTEGER NOT NULL,
    models TEXT NOT NULL, -- JSON blob of model configs
    max_concurrent INTEGER NOT NULL,
    hardware TEXT NOT NULL, -- JSON blob of hardware specs
    version TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Implementation Strategy

#### Single-Step Automatic Migration
The system will perform automatic migration on first startup with the new code:

1. **Add embedded SQLite database to the project**
2. **Create database schema and automatic migration system**
3. **Implement hybrid ConfigManager with automatic migration**
4. **System automatically migrates on startup and switches to hybrid mode**

#### Migration Process (Automatic)
```go
func (cm *ConfigManager) Initialize() error {
    // 1. Load static config from YAML (always required)
    staticConfig, err := cm.loadStaticConfig()
    if err != nil {
        return fmt.Errorf("failed to load static config: %w", err)
    }
    
    // 2. Initialize database
    if err := cm.initDatabase(); err != nil {
        return fmt.Errorf("failed to initialize database: %w", err)
    }
    
    // 3. Check if migration is needed (empty database = needs migration)
    if cm.needsMigration() {
        logging.Info("Performing automatic migration from YAML to hybrid system", types.Config)
        
        // 4. Migrate all dynamic data from YAML to database
        if err := cm.migrateFromYAML(staticConfig); err != nil {
            return fmt.Errorf("automatic migration failed: %w", err)
        }
        
        // 5. Clean up YAML - remove dynamic fields, keep only static config
        if err := cm.saveCleanStaticConfig(staticConfig); err != nil {
            logging.Warn("Failed to clean up YAML config", types.Config, "error", err)
        }
        
        logging.Info("Automatic migration completed successfully", types.Config)
    }
    
    return nil
}
```

#### Post-Migration Cleanup
1. **Update all callers** to use new ConfigManager API (no changes needed - same interface):
   - `node_handlers.go:37` - `syncNodesWithConfig()` calls
   - `node_handlers.go:65,120` - `config.SetNodes()` calls  
   - All height update locations that call `SetHeight()`
   - All seed update locations calling `SetCurrentSeed()`, etc.
2. **Dynamic data fields automatically removed from YAML** (`config.go:5,6,8-12,14,17-18`)
3. **Database backup/restore capabilities included**

### Technical Specifications

#### Database Selection: SQLite
- **Embedded**: No external dependencies
- **ACID Compliant**: Transactional guarantees
- **Cross-platform**: Works on all target architectures
- **Mature**: Battle-tested reliability
- **Lightweight**: Minimal overhead
- **Go Support**: Excellent CGO-free options available

#### Recommended Library: `modernc.org/sqlite`
- **Pure Go implementation** (no CGO required)
- **Embedded in binary** - no external database server needed
- **Full SQLite compatibility** with standard `database/sql` interface
- **Cross-platform** - works on all Go-supported architectures
- **Container-friendly** - perfect for Docker deployments
- **Zero external dependencies** - reduces build complexity
- **Single binary deployment** - database compiled into executable

```go
// Add to go.mod
require (
    modernc.org/sqlite v1.29.10
)

// Usage
import (
    "database/sql"
    _ "modernc.org/sqlite"  // Pure Go SQLite driver
)

func initDatabase(dbPath string) (*sql.DB, error) {
    return sql.Open("sqlite", dbPath)
}
```

#### Configuration Structure

```go
type HybridConfigManager struct {
    staticConfig StaticConfig
    database     *StateDatabase
    mutex        sync.RWMutex
}

type StaticConfig struct {
    Api             ApiConfig       `koanf:"api"`        // From config.go:4
    ChainNode       ChainNodeConfig `koanf:"chain_node"` // From config.go:7
    MLNodeKeyConfig MLNodeKeyConfig `koanf:"ml_node_key_config"` // From config.go:13
    Nats            NatsServerConfig `koanf:"nats"`      // From config.go:15
    // Note: Nodes (config.go:5) moved to database, NodeConfigIsMerged (config.go:6) no longer needed
}

type StateDatabase struct {
    db *sql.DB
    // Cached frequently accessed values
    currentHeight      atomic.Int64
    currentNodeVersion atomic.Value // string
    // ... other cached values
}

type DebugExporter struct {
    configManager *ConfigManager
    exportPath    string
    interval      time.Duration
    stopChan      chan struct{}
}
```

#### API Design

```go
// Read operations - source agnostic
func (cm *ConfigManager) GetHeight() int64
func (cm *ConfigManager) GetCurrentSeed() SeedInfo
func (cm *ConfigManager) GetValidationParams() ValidationParamsCache

// Write operations - automatically routed to appropriate storage
func (cm *ConfigManager) SetHeight(height int64) error                    // Currently: config_manager.go:104
func (cm *ConfigManager) SetCurrentSeed(seed SeedInfo) error              // Currently: config_manager.go:165
func (cm *ConfigManager) SetValidationParams(params ValidationParamsCache) error // Currently: config_manager.go:119
func (cm *ConfigManager) SetNodes(nodes []InferenceNodeConfig) error      // Currently: config_manager.go:185 -> database
func (cm *ConfigManager) AddNode(node InferenceNodeConfig) error          // New method -> database
func (cm *ConfigManager) RemoveNode(nodeId string) error                  // New method -> database

// Static config operations - YAML only
func (cm *ConfigManager) GetApiConfig() ApiConfig                         // Currently: config_manager.go:76
// Dynamic data operations - database only  
func (cm *ConfigManager) GetNodes() []InferenceNodeConfig                 // Currently: config_manager.go:84 <- database

// Debug/export operations
func (cm *ConfigManager) ExportFullConfig() (Config, error)          // merge static + dynamic
func (cm *ConfigManager) StartPeriodicExport(interval time.Duration) // background export
```

### Benefits

1. **Reliability**: Database ACID properties eliminate corruption risk
2. **Performance**: Targeted updates instead of full file rewrites
3. **Scalability**: Database can handle high-frequency updates efficiently  
4. **Maintainability**: Clear separation of concerns
5. **Observability**: Database queries provide better debugging capabilities
6. **Backup/Recovery**: Database-level backup and point-in-time recovery
7. **Concurrent Safety**: Database handles concurrent access properly
8. **REST API Efficiency**: Node CRUD operations (`node_handlers.go:23,71,89`) no longer require full config rewrites
9. **Transactional Node Updates**: Multiple node operations can be batched atomically (vs current `syncNodesWithConfig()` in `node_handlers.go:42`)
10. **Debug Visibility**: Periodic full config exports provide complete system state snapshots

### Automatic Migration Implementation

#### Migration Detection
```go
func (cm *HybridConfigManager) needsMigration() bool {
    // Check if database has been initialized with dynamic data
    var count int
    err := cm.database.db.QueryRow("SELECT COUNT(*) FROM chain_state").Scan(&count)
    if err != nil {
        // If table doesn't exist or query fails, we need migration
        return true
    }
    return count == 0 // Empty database = needs migration
}
```

#### Automatic Migration Function
```go
func (cm *HybridConfigManager) migrateFromYAML(originalConfig Config) error {
    // Create backup before migration
    backupPath := fmt.Sprintf("%s.pre-migration.%d", cm.configPath, time.Now().Unix())
    if err := cm.backupConfigFile(backupPath); err != nil {
        logging.Warn("Failed to create backup", types.Config, "error", err)
    }
    
    // Perform migration in single transaction
    tx, err := cm.database.db.Begin()
    if err != nil {
        return fmt.Errorf("failed to start migration transaction: %w", err)
    }
    defer tx.Rollback()
    
    // Migrate all dynamic data
    if err := cm.migrateChainState(tx, originalConfig); err != nil {
        return fmt.Errorf("failed to migrate chain state: %w", err)
    }
    if err := cm.migrateSeeds(tx, originalConfig); err != nil {
        return fmt.Errorf("failed to migrate seeds: %w", err)
    }
    if err := cm.migrateNetworkParams(tx, originalConfig); err != nil {
        return fmt.Errorf("failed to migrate network params: %w", err)
    }
    if err := cm.migrateNodes(tx, originalConfig.Nodes); err != nil {
        return fmt.Errorf("failed to migrate nodes: %w", err)
    }
    if err := cm.migrateNodeVersions(tx, originalConfig.NodeVersions); err != nil {
        return fmt.Errorf("failed to migrate node versions: %w", err)
    }
    if err := cm.migrateUpgradePlans(tx, originalConfig.UpgradePlan); err != nil {
        return fmt.Errorf("failed to migrate upgrade plans: %w", err)
    }
    
    // Commit all changes atomically
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit migration: %w", err)
    }
    
    logging.Info("Migration completed successfully", types.Config, 
        "backup_created", backupPath)
    return nil
}
```

#### Clean Config Generation
```go
func (cm *HybridConfigManager) saveCleanStaticConfig(originalConfig Config) error {
    cleanConfig := StaticConfig{
        Api:             originalConfig.Api,
        ChainNode:       originalConfig.ChainNode,
        MLNodeKeyConfig: originalConfig.MLNodeKeyConfig,
        Nats:            originalConfig.Nats,
    }
    
    // Write clean static-only config back to YAML
    return cm.writeStaticConfigToYAML(cleanConfig)
}
```

### Operational Considerations

#### Database Location
- Store in same directory as config file
- Use configurable path via environment variable
- Default: `{config_dir}/state.db`

#### Debug Export System
- **Full Config Export**: Merge static YAML + dynamic database state into complete config
- **Export Schedule**: Every 10 minutes (configurable via `CONFIG_EXPORT_INTERVAL`)
- **Export Location**: `{config_dir}/api-config-full.yaml` (timestamped backups optional)
- **Export Format**: Standard YAML format matching original config structure
- **Use Cases**: 
  - Debugging configuration state at specific points in time
  - Manual inspection of complete system configuration
  - Backup verification and disaster recovery testing
  - Support troubleshooting with complete config snapshot

#### Backup Strategy
- Automatic database backup before major operations
- Periodic snapshots for disaster recovery
- Export capability for configuration auditing
- **Periodic full dump**: Export complete config state to `api-config-full.yaml` every 10 minutes for debugging

#### Monitoring
- Database operation metrics
- Configuration change audit log
- Performance monitoring for database operations

#### Error Handling
- Graceful degradation when database unavailable
- Automatic database repair/recovery
- Clear error messages for troubleshooting

### Security Considerations

1. **File Permissions**: Restrict database file access (600)
2. **Database Encryption**: SQLite supports encryption via `PRAGMA key` for sensitive deployments
3. **Per-MLNode Security Keys**: Each MLNode can have individual security keys stored securely in database
4. **Audit Trail**: Log all configuration changes with timestamps
5. **Input Validation**: Strict validation before database writes
6. **Key Rotation**: Support for rotating MLNode security keys without config file changes

```sql
-- MLNode security keys table
CREATE TABLE mlnode_security (
    node_id TEXT PRIMARY KEY,
    security_key TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES inference_nodes(id)
);
```

### Testing Strategy

1. **Unit Tests**: Test database operations in isolation
2. **Integration Tests**: Test hybrid config manager behavior
3. **Migration Tests**: Verify smooth transition from YAML-only
4. **Corruption Tests**: Simulate failures and verify recovery
5. **Performance Tests**: Benchmark database vs file operations

### Rollback Plan

If issues arise during automatic migration:
1. **Automatic backup restoration**: Pre-migration backup is created at `{config}.pre-migration.{timestamp}`
2. **Manual rollback**: Restore backup file and restart with old binary
3. **Migration retry**: Fix issues and restart - migration will retry automatically
4. **Database reset**: Delete `state.db` file to force re-migration on next startup

```bash
# Emergency rollback procedure
cp config.yaml.pre-migration.1234567890 config.yaml
# Restart with old binary or delete state.db and restart with new binary
```

## Implementation Timeline

- **Week 1**: Database integration, schema creation, and automatic migration system
- **Week 2**: Hybrid ConfigManager implementation with seamless API compatibility  
- **Week 3**: Testing, validation, and production deployment preparation
- **Week 4**: Production deployment with automatic migration

## Current Code Analysis

Looking at the current code in `node_handlers.go`, the corruption-prone patterns are clear:

```go
// node_handlers.go:42 - syncNodesWithConfig()
func syncNodesWithConfig(nodeBroker *broker.Broker, config *apiconfig.ConfigManager) {
    nodes, err := nodeBroker.GetNodes()
    // ... convert nodes ...
    err = config.SetNodes(iNodes)  // Line 65 - triggers full file rewrite
}

// node_handlers.go:37 - called after every delete
syncNodesWithConfig(s.nodeBroker, s.configManager)

// node_handlers.go:120 - called after every add  
err = s.configManager.SetNodes(newNodes)
```

Each `SetNodes()` call triggers:
1. `config_manager.go:185` - `SetNodes()` method
2. `config_manager.go:188` - `writeConfig()` call
3. `config_manager.go:283` - Full config marshal and file truncate
4. `config_manager.go:228` - `os.O_TRUNC` flag opens file destructively

**The Problem**: Every REST API node operation (`POST /admin/v1/nodes`, `DELETE /admin/v1/nodes/:id`) rewrites the entire YAML configuration file, creating corruption risk during high-frequency operations.

**The Solution**: Automatic migration to hybrid system - move nodes and all dynamic data to database, eliminate file rewrites completely.

## Key Benefits of Automatic Migration

1. **Zero Operator Intervention**: Migration happens automatically on first startup
2. **Atomic Operation**: Either fully migrated or not - no partial states
3. **Automatic Backup**: Pre-migration backup created for safety
4. **Self-Healing**: Failed migrations can be retried by restarting
5. **Clean Separation**: Post-migration YAML contains only static configuration
6. **API Compatibility**: No changes needed to existing code using ConfigManager
7. **Rollback Safety**: Simple rollback procedure using automatic backups

This approach provides a robust, scalable solution that eliminates file corruption issues while maintaining operational simplicity through automatic migration.
