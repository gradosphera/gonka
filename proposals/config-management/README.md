# Configuration Management Redesign

## Problem

Current system rewrites entire YAML file for every config update:
- Height updates (every block)
- Seed updates (every epoch)
- Node registry changes (REST API)
- Parameters, versions, upgrade plans

Result: File corruption from concurrent writes, full rewrites for single field updates, no transaction guarantees.

## Solution: Hybrid Config

**YAML** - Static config (API ports, keys, endpoints, NATS config)
**SQLite** - Dynamic state (height, seeds, params, nodes, versions, upgrades)

```
┌─────────────────────┐    ┌──────────────────────┐
│   Static Config     │    │   Dynamic State DB   │
│   (config.yaml)     │    │   (embedded SQLite)  │
├─────────────────────┤    ├──────────────────────┤
│ • API Config        │    │ • Current Height     │
│ • Chain Node Config │    │ • Seed Information   │
│ • ML Node Keys      │    │ • Validation Params  │
│ • NATS Config       │    │ • Bandwidth Params   │
│                     │    │ • Node Versions      │
│                     │    │ • Upgrade Plans      │
│                     │    │ • Node Registry      │
└─────────────────────┘    └──────────────────────┘
          │                           │
          └─────────┬─────────────────┘
                    ▼
          ┌───────────────────┐
          │  ConfigManager    │
          └───────────────────┘
```

## Database Schema

```sql
CREATE TABLE chain_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE seed_info (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL, -- 'current', 'previous', 'upcoming'
    seed INTEGER NOT NULL,
    epoch_index INTEGER NOT NULL,
    signature TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE network_params (
    param_type TEXT PRIMARY KEY, -- 'validation', 'bandwidth'
    param_data TEXT NOT NULL, -- JSON blob
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE node_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    height INTEGER NOT NULL,
    version TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(height, version)
);

CREATE TABLE upgrade_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    height INTEGER NOT NULL,
    binaries TEXT NOT NULL, -- JSON blob
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inference_nodes (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    inference_segment TEXT NOT NULL,
    inference_port INTEGER NOT NULL,
    poc_segment TEXT NOT NULL,
    poc_port INTEGER NOT NULL,
    models TEXT NOT NULL, -- JSON blob
    max_concurrent INTEGER NOT NULL,
    hardware TEXT NOT NULL, -- JSON blob
    version TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE mlnode_security (
    node_id TEXT PRIMARY KEY,
    security_key TEXT NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES inference_nodes(id)
);
```

## Implementation

**Library**: `modernc.org/sqlite` (pure Go, no CGO, embedded)

**Automatic Migration on Startup**:
1. Load static config from YAML
2. Initialize database
3. If database empty, migrate dynamic data from YAML to database
4. Create backup before migration
5. Rewrite YAML with static config only

**Database Location**: `{config_dir}/state.db`

**API Compatibility**: Existing ConfigManager methods unchanged

**Rollback**: `cp config.yaml.pre-migration.{timestamp} config.yaml`

## Debug Export

Merge static YAML + database state into `api-config-full.yaml` every 10 minutes for debugging/troubleshooting.
