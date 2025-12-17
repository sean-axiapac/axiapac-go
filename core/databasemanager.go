package core

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseManager struct {
	sqlDB *sql.DB
}

// New creates the global pool (e.g. 30 conns).
// dsn should NOT include schema (just host/user/pass).
func New(dsn string, maxConnection int) (*DatabaseManager, error) {
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open pool: %w", err)
	}

	sqlDB.SetMaxOpenConns(maxConnection)
	sqlDB.SetMaxIdleConns(maxConnection)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping pool: %w", err)
	}

	return &DatabaseManager{sqlDB: sqlDB}, nil
}

// GetDB gets a *gorm.DB bound to a single connection
// and sets the schema with `USE schema`.
func (p *DatabaseManager) GetDB(ctx context.Context, schema string) (*gorm.DB, *sql.Conn, error) {
	if schema == "localhost" {
		dsn := os.Getenv("DSN")

		// Split on "?" to remove query params
		parts := strings.SplitN(dsn, "?", 2)
		dsnWithoutQuery := parts[0]

		// Split on "/" to get DB name (last part)
		segments := strings.Split(dsnWithoutQuery, "/")
		schema = segments[len(segments)-1]
	}

	// Get a dedicated connection from pool
	conn, err := p.sqlDB.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get conn: %w", err)
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	// Switch schema
	if _, err := conn.ExecContext(ctx, "USE `"+schema+"`"); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to use schema %s: %w", schema, err)
	}

	// Wrap this single connection into GORM
	dialector := mysql.New(mysql.Config{
		Conn: conn, // lock GORM to this connection
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to open gorm: %w", err)
	}

	// cancel the deferred close; caller will close
	defer func() { conn = nil }()
	return db, conn, nil
}

// Close closes the global pool
func (p *DatabaseManager) Close() error {
	return p.sqlDB.Close()
}

func (dm *DatabaseManager) Exec(ctx context.Context, schema string, fn func(db *gorm.DB) error) error {
	db, conn, err := dm.GetDB(ctx, schema)
	if err != nil {
		return err
	}
	defer conn.Close()

	return fn(db)
}
