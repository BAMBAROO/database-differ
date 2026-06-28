package connector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connector defines operations to establish and validate database connections.
type Connector interface {
	Connect(dsn string) (*sql.DB, error)
	ValidateReadPrivilege(db *sql.DB) error
}

type BaseConnector struct {
	Driver       string
	RetryCount   int
	RetryBackoff time.Duration
}

// NewConnector creates a connector based on the driver.
func NewConnector(driver string) (Connector, error) {
	switch driver {
	case "mysql":
		return &MySQLConnector{
			BaseConnector: BaseConnector{
				Driver:       "mysql",
				RetryCount:   3,
				RetryBackoff: 500 * time.Millisecond,
			},
		}, nil
	case "postgres":
		return &PostgreSQLConnector{
			BaseConnector: BaseConnector{
				Driver:       "pgx", // Using pgx stdlib
				RetryCount:   3,
				RetryBackoff: 500 * time.Millisecond,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// connectWithRetry implements connection retry logic common to both databases.
func (bc *BaseConnector) connectWithRetry(driver, dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error

	for i := 0; i < bc.RetryCount; i++ {
		db, err = sql.Open(driver, dsn)
		if err == nil {
			// Enforce a strict 2-second timeout per Ping attempt
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				return db, nil
			}
		}

		if db != nil {
			_ = db.Close()
		}

		if i < bc.RetryCount-1 {
			time.Sleep(bc.RetryBackoff * time.Duration(1<<i))
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", bc.RetryCount, err)
}

// MySQLConnector handles MySQL connections.
type MySQLConnector struct {
	BaseConnector
}

func (c *MySQLConnector) Connect(dsn string) (*sql.DB, error) {
	return c.connectWithRetry("mysql", dsn)
}

func (c *MySQLConnector) ValidateReadPrivilege(db *sql.DB) error {
	// Heuristic privilege check: Perform a query on INFORMATION_SCHEMA
	var val int
	query := "SELECT 1 FROM INFORMATION_SCHEMA.TABLES LIMIT 1"
	err := db.QueryRow(query).Scan(&val)
	if err != nil {
		return fmt.Errorf("failed privilege validation: read access to INFORMATION_SCHEMA.TABLES denied. Make sure user has SELECT privileges. Error: %w", err)
	}
	return nil
}

// PostgreSQLConnector handles PostgreSQL connections.
type PostgreSQLConnector struct {
	BaseConnector
}

func (c *PostgreSQLConnector) Connect(dsn string) (*sql.DB, error) {
	return c.connectWithRetry("pgx", dsn)
}

func (c *PostgreSQLConnector) ValidateReadPrivilege(db *sql.DB) error {
	// Heuristic privilege check: Perform a query on information_schema.tables
	var val int
	query := "SELECT 1 FROM information_schema.tables LIMIT 1"
	err := db.QueryRow(query).Scan(&val)
	if err != nil {
		return fmt.Errorf("failed privilege validation: read access to information_schema.tables denied. Error: %w", err)
	}
	return nil
}
