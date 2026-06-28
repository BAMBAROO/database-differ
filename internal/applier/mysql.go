package applier

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

type MySQLApplier struct{}

func NewMySQLApplier() Applier {
	return &MySQLApplier{}
}

type MySQLMigrationState struct {
	GeneratedAt string               `json:"generated_at"`
	Driver      string               `json:"driver"`
	Statements  []MySQLStatementState `json:"statements"`
}

type MySQLStatementState struct {
	ID     int    `json:"id"`
	SQL    string `json:"sql"`
	Status string `json:"status"` // pending, done, failed
}

func (a *MySQLApplier) Apply(db *sql.DB, statements []string, stateFilePath string, resume bool) error {
	if stateFilePath == "" {
		stateFilePath = "migration_state.json"
	}

	var state MySQLMigrationState
	var err error

	// Filter out comments and empty statements from the input slice
	var executableStmts []string
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		executableStmts = append(executableStmts, trimmed)
	}

	if resume {
		// Read state file
		stateData, err := os.ReadFile(stateFilePath)
		if err != nil {
			return fmt.Errorf("failed to read state file for resume: %w", err)
		}
		if err := json.Unmarshal(stateData, &state); err != nil {
			return fmt.Errorf("failed to parse state file for resume: %w", err)
		}
		fmt.Printf("Resuming migration from state file: %s\n", stateFilePath)
	} else {
		// Initialize state
		state = MySQLMigrationState{
			GeneratedAt: time.Now().Format(time.RFC3339),
			Driver:      "mysql",
			Statements:  []MySQLStatementState{},
		}
		for i, stmt := range executableStmts {
			state.Statements = append(state.Statements, MySQLStatementState{
				ID:     i + 1,
				SQL:    stmt,
				Status: "pending",
			})
		}

		// Write initial state to file
		if err := writeStateFile(stateFilePath, state); err != nil {
			return fmt.Errorf("failed to write initial state file: %w", err)
		}
	}

	totalPending := 0
	for _, s := range state.Statements {
		if s.Status == "pending" || s.Status == "failed" {
			totalPending++
		}
	}

	if totalPending == 0 {
		fmt.Println("All statements have already been applied successfully.")
		_ = os.Remove(stateFilePath) // clean up completed state file
		return nil
	}

	fmt.Printf("Applying %d DDL statements to MySQL target database...\n", totalPending)

	// Setup progress bar
	bar := progressbar.Default(int64(totalPending), "Executing")

	for idx, stmtState := range state.Statements {
		if stmtState.Status == "done" {
			continue
		}

		// Execute SQL statement
		_, err = db.Exec(stmtState.SQL)
		if err != nil {
			// Mark as failed and write state
			state.Statements[idx].Status = "failed"
			_ = writeStateFile(stateFilePath, state)

			// Formatting user friendly error message
			return fmt.Errorf(
				"\n✗ Failed at statement #%d\n"+
					"  SQL: %s\n"+
					"  Error: %v\n\n"+
					"⚠️  MySQL Warning: Prior DDL statements in this migration run have already been applied and cannot be rolled back.\n"+
					"✦  Run with --resume to continue from statement #%d after fixing the issue.\n"+
					"✦  State file saved to: %s",
				stmtState.ID, stmtState.SQL, err, stmtState.ID, stateFilePath,
			)
		}

		// Update state to done and save
		state.Statements[idx].Status = "done"
		if err := writeStateFile(stateFilePath, state); err != nil {
			return fmt.Errorf("failed to update state file: %w", err)
		}

		_ = bar.Add(1)
	}

	fmt.Println("\nAll statements applied successfully.")
	// Delete state file upon successful completion
	_ = os.Remove(stateFilePath)

	return nil
}

func writeStateFile(path string, state MySQLMigrationState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
