package reporter

import (
	"io"

	"github.com/bryanathallah/db-schema-differ/models"
)

// Reporter defines the contract for formatting and printing schema diff results.
type Reporter interface {
	Report(diff *models.SchemaDiff, writer io.Writer) error
}
