package applier

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/schollz/progressbar/v3"
)

type PostgresApplier struct{}

func NewPostgresApplier() Applier {
	return &PostgresApplier{}
}

func (a *PostgresApplier) Apply(db *sql.DB, statements []string, stateFilePath string, resume bool) error {
	// Filter out comments, empty statements, and transaction boundary strings (since we use db.Begin)
	var executableStmts []string
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if upper == "BEGIN;" || upper == "BEGIN" || upper == "COMMIT;" || upper == "COMMIT" || upper == "ROLLBACK;" || upper == "ROLLBACK" {
			continue
		}
		executableStmts = append(executableStmts, trimmed)
	}

	if len(executableStmts) == 0 {
		fmt.Println("No statements to apply.")
		return nil
	}

	fmt.Printf("Applying %d DDL statements inside a PostgreSQL transaction...\n", len(executableStmts))

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start database transaction: %w", err)
	}

	// Setup progress bar
	bar := progressbar.Default(int64(len(executableStmts)), "Executing")

	for i, stmt := range executableStmts {
		_, err := tx.ExecContext(ctx, stmt)
		if err != nil {
			// Rollback transaction immediately
			rbErr := tx.Rollback()
			if rbErr != nil {
				return fmt.Errorf("failed at statement #%d: %v (failed to rollback: %v)", i+1, err, rbErr)
			}
			return fmt.Errorf("failed at statement #%d: %v (transaction rolled back)", i+1, err)
		}
		_ = bar.Add(1)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("\nAll statements applied successfully (transaction committed).")
	return nil
}
