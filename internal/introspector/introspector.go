package introspector

import (
	"database/sql"

	"github.com/bryanathallah/db-schema-differ/models"
)

// Introspector defines the contract for reading database schemas.
type Introspector interface {
	Introspect(db *sql.DB, dbName string) (*models.Schema, error)
}
