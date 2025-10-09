package apiconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// SqliteConfig holds configuration for an embedded SQLite DB
type SqliteConfig struct {
	Path string // e.g., gonka.db
}

type SqlDatabase interface {
	BootstrapLocal(ctx context.Context) error
	GetDb() *sql.DB
}

type SqliteDb struct {
	config SqliteConfig
	db     *sql.DB
}

func NewSQLiteDb(cfg SqliteConfig) *SqliteDb {
	return &SqliteDb{config: cfg}
}

func (d *SqliteDb) BootstrapLocal(ctx context.Context) error {
	db, err := OpenSQLite(d.config)
	if err != nil {
		return err
	}
	if err := EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	d.db = db
	return nil
}

func (d *SqliteDb) GetDb() *sql.DB { return d.db }

// OpenSQLite opens an embedded SQLite database (in process)
func OpenSQLite(cfg SqliteConfig) (*sql.DB, error) {
	if cfg.Path == "" {
		return nil, errors.New("sqlite path is empty")
	}
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, err
	}
	// Reasonable pool defaults for sqlite
	db.SetMaxOpenConns(1) // SQLite is single-writer
	db.SetConnMaxLifetime(0)
	return db, nil
}

// EnsureSchema creates the minimal tables for storing dynamic config: inference nodes.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmt := `
CREATE TABLE IF NOT EXISTS inference_nodes (
  id TEXT PRIMARY KEY,
  host TEXT NOT NULL,
  inference_segment TEXT NOT NULL,
  inference_port INTEGER NOT NULL,
  poc_segment TEXT NOT NULL,
  poc_port INTEGER NOT NULL,
  max_concurrent INTEGER NOT NULL,
  models_json TEXT NOT NULL,
  hardware_json TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%f','now')),
  created_at DATETIME NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE IF NOT EXISTS kv_config (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%f','now')),
  created_at DATETIME NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE IF NOT EXISTS seed_info (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL, -- 'current', 'previous', 'upcoming'
  seed INTEGER NOT NULL,
  epoch_index INTEGER NOT NULL,
  signature TEXT NOT NULL,
  claimed BOOLEAN NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%f','now'))
);`
	_, err := db.ExecContext(ctx, stmt)
	return err
}

// UpsertInferenceNodes replaces or inserts the given nodes by id.
func UpsertInferenceNodes(ctx context.Context, db *sql.DB, nodes []InferenceNodeConfig) error {
	if len(nodes) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	q := `
INSERT INTO inference_nodes (
  id, host, inference_segment, inference_port, poc_segment, poc_port, max_concurrent, models_json, hardware_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  host = excluded.host,
  inference_segment = excluded.inference_segment,
  inference_port = excluded.inference_port,
  poc_segment = excluded.poc_segment,
  poc_port = excluded.poc_port,
  max_concurrent = excluded.max_concurrent,
  models_json = excluded.models_json,
  hardware_json = excluded.hardware_json`

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, n := range nodes {
		modelsJSON, err := json.Marshal(n.Models)
		if err != nil {
			return err
		}
		hardwareJSON, err := json.Marshal(n.Hardware)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(
			ctx,
			n.Id,
			n.Host,
			n.InferenceSegment,
			n.InferencePort,
			n.PoCSegment,
			n.PoCPort,
			n.MaxConcurrent,
			string(modelsJSON),
			string(hardwareJSON),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// WriteNodes is a convenience wrapper for UpsertInferenceNodes.
func WriteNodes(ctx context.Context, db *sql.DB, nodes []InferenceNodeConfig) error {
	return UpsertInferenceNodes(ctx, db, nodes)
}

// ReadNodes reads all nodes from the database and reconstructs InferenceNodeConfig entries.
func ReadNodes(ctx context.Context, db *sql.DB) ([]InferenceNodeConfig, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, host, inference_segment, inference_port, poc_segment, poc_port, max_concurrent, models_json, hardware_json
FROM inference_nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InferenceNodeConfig
	for rows.Next() {
		var (
			id          string
			host        string
			infSeg      string
			infPort     int
			pocSeg      string
			pocPort     int
			maxConc     int
			modelsRaw   []byte
			hardwareRaw []byte
		)
		if err := rows.Scan(&id, &host, &infSeg, &infPort, &pocSeg, &pocPort, &maxConc, &modelsRaw, &hardwareRaw); err != nil {
			return nil, err
		}
		var models map[string]ModelConfig
		if len(modelsRaw) > 0 {
			if err := json.Unmarshal(modelsRaw, &models); err != nil {
				return nil, err
			}
		}
		var hardware []Hardware
		if len(hardwareRaw) > 0 {
			if err := json.Unmarshal(hardwareRaw, &hardware); err != nil {
				return nil, err
			}
		}
		out = append(out, InferenceNodeConfig{
			Host:             host,
			InferenceSegment: infSeg,
			InferencePort:    infPort,
			PoCSegment:       pocSeg,
			PoCPort:          pocPort,
			Models:           models,
			Id:               id,
			MaxConcurrent:    maxConc,
			Hardware:         hardware,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ReplaceInferenceNodes deletes all nodes and inserts the given list atomically.
func ReplaceInferenceNodes(ctx context.Context, db *sql.DB, nodes []InferenceNodeConfig) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM inference_nodes`); err != nil {
		return err
	}

	if len(nodes) == 0 {
		return tx.Commit()
	}

	q := `
INSERT INTO inference_nodes (
  id, host, inference_segment, inference_port, poc_segment, poc_port, max_concurrent, models_json, hardware_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, n := range nodes {
		modelsJSON, err := json.Marshal(n.Models)
		if err != nil {
			return err
		}
		hardwareJSON, err := json.Marshal(n.Hardware)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(
			ctx,
			n.Id,
			n.Host,
			n.InferenceSegment,
			n.InferencePort,
			n.PoCSegment,
			n.PoCPort,
			n.MaxConcurrent,
			string(modelsJSON),
			string(hardwareJSON),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeedInfo typed accessors

// SetActiveSeed deactivates previous active seed of given type and inserts a new active row.
func SetActiveSeed(ctx context.Context, db *sql.DB, seedType string, info SeedInfo) error {
	if db == nil {
		return errors.New("db is nil")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE seed_info SET is_active = 0 WHERE type = ? AND is_active = 1`, seedType); err != nil {
		return err
	}
	q := `INSERT INTO seed_info(type, seed, epoch_index, signature, claimed, is_active) VALUES(?, ?, ?, ?, ?, 1)`
	if _, err := tx.ExecContext(ctx, q, seedType, info.Seed, info.EpochIndex, info.Signature, info.Claimed); err != nil {
		return err
	}
	return tx.Commit()
}

// GetActiveSeed returns the active seed for type; ok=false if none.
func GetActiveSeed(ctx context.Context, db *sql.DB, seedType string) (SeedInfo, bool, error) {
	if db == nil {
		return SeedInfo{}, false, errors.New("db is nil")
	}
	row := db.QueryRowContext(ctx, `SELECT seed, epoch_index, signature, claimed FROM seed_info WHERE type = ? AND is_active = 1 ORDER BY id DESC LIMIT 1`, seedType)
	var s SeedInfo
	if err := row.Scan(&s.Seed, &s.EpochIndex, &s.Signature, &s.Claimed); err != nil {
		if err == sql.ErrNoRows {
			return SeedInfo{}, false, nil
		}
		return SeedInfo{}, false, err
	}
	return s, true, nil
}

// MarkSeedClaimed sets claimed=true for current active seed of given type. ok=false if none.
func MarkSeedClaimed(ctx context.Context, db *sql.DB, seedType string) (ok bool, err error) {
	if db == nil {
		return false, errors.New("db is nil")
	}
	res, err := db.ExecContext(ctx, `UPDATE seed_info SET claimed = 1 WHERE type = ? AND is_active = 1`, seedType)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

// IsSeedClaimed reads claimed for active seed of given type. ok=false if none.
func IsSeedClaimed(ctx context.Context, db *sql.DB, seedType string) (claimed bool, ok bool, err error) {
	if db == nil {
		return false, false, errors.New("db is nil")
	}
	row := db.QueryRowContext(ctx, `SELECT claimed FROM seed_info WHERE type = ? AND is_active = 1 ORDER BY id DESC LIMIT 1`, seedType)
	var c bool
	if err := row.Scan(&c); err != nil {
		if err == sql.ErrNoRows {
			return false, false, nil
		}
		return false, false, err
	}
	return c, true, nil
}

// KV helpers for dynamic config

// KVSetJSON upserts an arbitrary Go value encoded as JSON at the given key.
func KVSetJSON(ctx context.Context, db *sql.DB, key string, value any) error {
	if db == nil {
		return errors.New("db is nil")
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	q := `INSERT INTO kv_config(key, value_json) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = (STRFTIME('%Y-%m-%d %H:%M:%f','now'))`
	if _, err := tx.ExecContext(ctx, q, key, string(bytes)); err != nil {
		return err
	}
	return tx.Commit()
}

// KVGetJSON loads a key and unmarshals JSON into destPtr.
// If key not found, ok=false and no error is returned.
func KVGetJSON(ctx context.Context, db *sql.DB, key string, destPtr any) (ok bool, err error) {
	if db == nil {
		return false, errors.New("db is nil")
	}
	var raw string
	err = db.QueryRowContext(ctx, `SELECT value_json FROM kv_config WHERE key = ?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), destPtr); err != nil {
		return false, fmt.Errorf("unmarshal json for key %s: %w", key, err)
	}
	return true, nil
}

// KVSetInt64 stores an int64 under key.
func KVSetInt64(ctx context.Context, db *sql.DB, key string, v int64) error {
	return KVSetJSON(ctx, db, key, v)
}

// KVGetInt64 retrieves an int64. If missing, returns ok=false.
func KVGetInt64(ctx context.Context, db *sql.DB, key string) (val int64, ok bool, err error) {
	var tmp int64
	ok, err = KVGetJSON(ctx, db, key, &tmp)
	if !ok || err != nil {
		return 0, ok, err
	}
	return tmp, true, nil
}

// KVSetString stores a string under key.
func KVSetString(ctx context.Context, db *sql.DB, key string, v string) error {
	return KVSetJSON(ctx, db, key, v)
}

// KVGetString retrieves a string. If missing, returns ok=false.
func KVGetString(ctx context.Context, db *sql.DB, key string) (val string, ok bool, err error) {
	var tmp string
	ok, err = KVGetJSON(ctx, db, key, &tmp)
	if !ok || err != nil {
		return "", ok, err
	}
	return tmp, true, nil
}
