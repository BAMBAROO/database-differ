package reporter

import (
	"encoding/json"
	"io"

	"github.com/bryanathallah/db-schema-differ/models"
)

type JSONReporter struct{}

func NewJSONReporter() Reporter {
	return &JSONReporter{}
}

func (r *JSONReporter) Report(diff *models.SchemaDiff, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diff)
}
