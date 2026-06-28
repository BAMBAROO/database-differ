package applier

import (
	"database/sql"
)

// Applier defines the interface to execute generated DDL statements on the database.
type Applier interface {
	Apply(db *sql.DB, statements []string, stateFilePath string, resume bool) error
}
