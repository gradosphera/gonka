package apiconfig

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySqlConfig holds connection params for a MySQL database.
type MySqlConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
	Params   map[string]string
	// If set, connect via Unix domain socket instead of TCP, e.g. /tmp/mysql.sock
	UnixSocket string
}

type SqlDatabase interface {
	BootstrapLocal(ctx context.Context) error
	GetDb() *sql.DB
}

type MySqlDb struct {
	config MySqlConfig
	db     *sql.DB
}

func NewMySQLDb(cfg MySqlConfig) *MySqlDb {
	return &MySqlDb{config: cfg}
}

func (d *MySqlDb) BootstrapLocal(ctx context.Context) error {
	// Try normal connect; if db missing, create it by connecting without a default DB.
	appDB, err := OpenMySQL(d.config)
	if err != nil {
		return err
	}
	if pingErr := appDB.PingContext(ctx); pingErr == nil {
		if err := EnsureSchema(ctx, appDB); err != nil {
			_ = appDB.Close()
			return err
		}
		d.db = appDB
		return nil
	} else if isUnknownDatabase(pingErr) {
		_ = appDB.Close()
		// Reconnect without database and create it
		temp := d.config
		temp.Database = ""
		noDB, err2 := OpenMySQL(temp)
		if err2 != nil {
			return err2
		}
		if err := createDatabaseIfNotExists(ctx, noDB, d.config.Database); err != nil {
			_ = noDB.Close()
			return err
		}
		_ = noDB.Close()
		// Connect again to the newly created DB and ensure schema
		withDB, err3 := OpenMySQL(d.config)
		if err3 != nil {
			return err3
		}
		if err := withDB.PingContext(ctx); err != nil {
			_ = withDB.Close()
			return err
		}
		if err := EnsureSchema(ctx, withDB); err != nil {
			_ = withDB.Close()
			return err
		}
		d.db = withDB
		return nil
	} else {
		_ = appDB.Close()
		return pingErr
	}
}

func (d *MySqlDb) GetDb() *sql.DB { return d.db }

// BuildDSN constructs a DSN string for go-sql-driver/mysql using only pure Go bits.
func (c MySqlConfig) BuildDSN() string {
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
	netSpec := ""
	if strings.TrimSpace(c.UnixSocket) != "" {
		netSpec = fmt.Sprintf("unix(%s)", c.UnixSocket)
	} else {
		netSpec = fmt.Sprintf("tcp(%s:%d)", c.Host, c.Port)
	}
	return fmt.Sprintf("%s:%s@%s/%s%s", c.Username, c.Password, netSpec, c.Database, paramStr)
}

// OpenMySQL opens a database handle with sane defaults.
func OpenMySQL(cfg MySqlConfig) (*sql.DB, error) {
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

// Helpers to detect common MySQL errors without stringly-typed checks everywhere.
func isAccessDenied(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1045 // ER_ACCESS_DENIED_ERROR
	}
	// Fallback on message contains when driver wraps differently
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "access denied")
}

func isUnknownDatabase(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1049 // ER_BAD_DB_ERROR
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unknown database")
}

func createDatabaseIfNotExists(ctx context.Context, db *sql.DB, dbName string) error {
	if strings.TrimSpace(dbName) == "" {
		return errors.New("empty database name")
	}
	_, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+dbName+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
	return err
}

func createUserIfNotExists(ctx context.Context, db *sql.DB, user, pass string) error {
	if strings.TrimSpace(user) == "" {
		return errors.New("empty user name")
	}
	// MySQL 8: CREATE USER IF NOT EXISTS and set password
	_, err := db.ExecContext(ctx, "CREATE USER IF NOT EXISTS `"+user+"` IDENTIFIED BY '"+pass+"'")
	return err
}

func grantAllOnDB(ctx context.Context, db *sql.DB, user, dbName string) error {
	if strings.TrimSpace(user) == "" || strings.TrimSpace(dbName) == "" {
		return errors.New("empty user or database name")
	}
	_, err := db.ExecContext(ctx, "GRANT ALL PRIVILEGES ON `"+dbName+"`.* TO `"+user+"`")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "FLUSH PRIVILEGES")
	return err
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
