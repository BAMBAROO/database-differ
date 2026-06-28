package generator

import (
	"github.com/bryanathallah/db-schema-differ/models"
)

// GenOptions contains configuration for code generators.
type GenOptions struct {
	SafeOnly bool
}

// Generator defines the interface for generating DDL statements.
type Generator interface {
	Generate(diff *models.SchemaDiff, sourceSchema *models.Schema, options GenOptions) ([]string, error)
}
