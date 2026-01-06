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

type LogLevel int

const (
	LogLevelSilent LogLevel = iota + 1
	LogLevelError
	LogLevelWarn
	LogLevelInfo
)

type DatabaseManager struct {
	SqlDB    *sql.DB
	LogLevel LogLevel
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

	return &DatabaseManager{SqlDB: sqlDB}, nil
}

// GetDB gets a *gorm.DB bound to a single connection
// and sets the schema with `USE schema`.
func (dm *DatabaseManager) GetDB(ctx context.Context, schema string) (*gorm.DB, *sql.Conn, error) {
	if schema == "localhost" {
		dsn := os.Getenv("DSN")

		// Split on "?" to remove query params
		parts := strings.SplitN(dsn, "?", 2)
		dsnWithoutQuery := parts[0]

		// Split on "/" to get DB name (last part)
		segments := strings.Split(dsnWithoutQuery, "/")
		schema = segments[len(segments)-1]
	} else {
		// splite by "." and take the first. e.g. "oktedi.axiapac.net.au" -> "oktedi"
		parts := strings.Split(schema, ".")
		schema = parts[0]
	}

	// Get a dedicated connection from pool
	conn, err := dm.SqlDB.Conn(ctx)
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
	// Map local LogLevel to GORM LogLevel
	gormLogLevel := logger.Silent
	switch dm.LogLevel {
	case LogLevelError:
		gormLogLevel = logger.Error
	case LogLevelWarn:
		gormLogLevel = logger.Warn
	case LogLevelInfo:
		gormLogLevel = logger.Info
	case LogLevelSilent:
		gormLogLevel = logger.Silent
	default:
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
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
func (dm *DatabaseManager) Close() error {
	return dm.SqlDB.Close()
}

func (dm *DatabaseManager) Exec(ctx context.Context, schema string, fn func(db *gorm.DB) error) error {
	db, conn, err := dm.GetDB(ctx, schema)
	if err != nil {
		return err
	}
	defer conn.Close()

	return fn(db)
}

func (dm *DatabaseManager) GetAllDatabases(ctx context.Context) ([]string, error) {
	rows, err := dm.SqlDB.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		// Filter out system databases
		switch db {
		case "information_schema", "mysql", "performance_schema", "sys":
			continue
		}
		databases = append(databases, db)
	}

	return databases, nil
}
