package apiconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfig holds connection params for a MySQL database.
type MySQLConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
	Params   map[string]string
}

// BuildDSN constructs a DSN string for go-sql-driver/mysql using only pure Go bits.
func (c MySQLConfig) BuildDSN() string {
	// Default tcp connection
	paramStr := ""
	if len(c.Params) > 0 {
		first := true
		for k, v := range c.Params {
			if first {
				paramStr += fmt.Sprintf("%s=%s", k, v)
				first = false
				continue
			}
			paramStr += fmt.Sprintf("&%s=%s", k, v)
		}
	}
	if paramStr != "" {
		paramStr = "?" + paramStr
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s%s", c.Username, c.Password, c.Host, c.Port, c.Database, paramStr)
}

// OpenMySQL opens a database handle with sane defaults.
func OpenMySQL(cfg MySQLConfig) (*sql.DB, error) {
	dsn := cfg.BuildDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// Reasonable pool defaults
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// EnsureSchema creates the minimal tables for storing dynamic config: inference nodes and models.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	// Nodes table: host+id unique, plus other fields; models/hardware stored as JSON for simplicity.
	// If you want normalized tables, you can split into two tables as needed later.
	stmt := `
CREATE TABLE IF NOT EXISTS inference_nodes (
  id VARCHAR(191) NOT NULL,
  host VARCHAR(191) NOT NULL,
  inference_segment VARCHAR(255) NOT NULL,
  inference_port INT NOT NULL,
  poc_segment VARCHAR(255) NOT NULL,
  poc_port INT NOT NULL,
  max_concurrent INT NOT NULL,
  models_json JSON NOT NULL,
  hardware_json JSON NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_host (host)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;`
	_, err := db.ExecContext(ctx, stmt)
	return err
}

// UpsertInferenceNodes replaces or inserts the given nodes by id.
func UpsertInferenceNodes(ctx context.Context, db *sql.DB, nodes []InferenceNodeConfig) error {
	if len(nodes) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		// Rollback if still active
		_ = tx.Rollback()
	}()

	q := `
INSERT INTO inference_nodes (
  id, host, inference_segment, inference_port, poc_segment, poc_port, max_concurrent, models_json, hardware_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
AS new
ON DUPLICATE KEY UPDATE
  host = new.host,
  inference_segment = new.inference_segment,
  inference_port = new.inference_port,
  poc_segment = new.poc_segment,
  poc_port = new.poc_port,
  max_concurrent = new.max_concurrent,
  models_json = new.models_json,
  hardware_json = new.hardware_json`

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

// ExampleWriteNodes is an example helper showing how to connect and write nodes to MySQL.
// It is placed in the same package and directory as config.go as requested.
func ExampleWriteNodes(ctx context.Context, dbHost string, dbPort int, dbUser, dbPass, dbName string, nodes []InferenceNodeConfig) error {
	cfg := MySQLConfig{
		Username: dbUser,
		Password: dbPass,
		Host:     dbHost,
		Port:     dbPort,
		Database: dbName,
		Params: map[string]string{
			"parseTime": "true",
			"charset":   "utf8mb4",
			"collation": "utf8mb4_0900_ai_ci",
		},
	}
	db, err := OpenMySQL(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := EnsureSchema(ctx, db); err != nil {
		return err
	}
	return UpsertInferenceNodes(ctx, db, nodes)
}
