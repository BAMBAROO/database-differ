package models

// Schema represents the complete structure of a database.
type Schema struct {
	Name   string           `json:"name"`
	Tables map[string]Table `json:"tables"`
}

// Table represents a database table schema.
type Table struct {
	Name        string                `json:"name"`
	Engine      string                `json:"engine,omitempty"`       // MySQL engine (e.g. InnoDB)
	Collation   string                `json:"collation,omitempty"`    // Character set / Collation
	Comment     string                `json:"comment,omitempty"`      // Table description
	Columns     map[string]Column     `json:"columns"`
	ColumnOrder []string              `json:"column_order"`           // To maintain the column layout order
	Indexes     map[string]Index      `json:"indexes"`
	ForeignKeys map[string]ForeignKey `json:"foreign_keys"`
}

// Column represents a table column schema.
type Column struct {
	Name           string  `json:"name"`
	NormalizedType string  `json:"normalized_type"` // Normalized database-agnostic type string for comparison
	RawType        string  `json:"raw_type"`        // Original database-specific type (for generation / debug)
	Nullable       string  `json:"nullable"`        // YES or NO
	DefaultValue   *string `json:"default_value"`   // Pointer to handle NULL default value distinction
	AutoIncrement  bool    `json:"auto_increment"`
	Collation      string  `json:"collation,omitempty"`
	Comment        string  `json:"comment,omitempty"`
	Position       int     `json:"position"` // 1-indexed order position in the table
}

// Index represents a table index schema.
type Index struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`    // PRIMARY, UNIQUE, FULLTEXT, INDEX
	Columns []string `json:"columns"` // Column names in order
}

// ForeignKey represents a table foreign key constraint schema.
type ForeignKey struct {
	Name             string `json:"name"`
	Column           string `json:"column"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
	OnDelete         string `json:"on_delete"`
	OnUpdate         string `json:"on_update"`
}

// DiffSeverity represents the risk level of a schema change.
type DiffSeverity string

const (
	SeveritySafe    DiffSeverity = "SAFE"    // Addition of table, column, index, FK
	SeverityWarning DiffSeverity = "WARNING" // Modification of column type, length, nullability (potential truncation/conversion errors)
	SeverityDanger  DiffSeverity = "DANGER"  // Drop of table, column, index, FK
)

// DiffAction represents the type of change.
type DiffAction string

const (
	ActionAdd    DiffAction = "ADD"
	ActionModify DiffAction = "MODIFY"
	ActionDrop   DiffAction = "DROP"
)

// SchemaDiff represents the final calculated differences between two schemas.
type SchemaDiff struct {
	Summary SchemaDiffSummary `json:"summary"`
	Tables  []TableDiff       `json:"tables"`
}

// SchemaDiffSummary holds count statistics for the schema differences.
type SchemaDiffSummary struct {
	SourceDB       string `json:"source_db"`
	TargetDB       string `json:"target_db"`
	TablesAdded    int    `json:"tables_added"`
	TablesModified int    `json:"tables_modified"`
	TablesDropped  int    `json:"tables_dropped"`
	ColumnsAdded   int    `json:"columns_added"`
	ColumnsModified int    `json:"columns_modified"`
	ColumnsDropped  int    `json:"columns_dropped"`
	IndexesAdded   int    `json:"indexes_added"`
	IndexesDropped int    `json:"indexes_dropped"`
	FKsAdded       int    `json:"fks_added"`
	FKsDropped     int    `json:"fks_dropped"`
	WarningsCount  int    `json:"warnings_count"`
	DangersCount   int    `json:"dangers_count"`
}

// TableDiff represents changes to a specific table.
type TableDiff struct {
	TableName string       `json:"table_name"`
	Action    DiffAction   `json:"action"` // ADD, MODIFY, DROP
	Severity  DiffSeverity `json:"severity"`
	Comment   string       `json:"comment,omitempty"` // E.g., "Table added", "Table dropped"

	// Only populated for MODIFY actions:
	Columns     []ColumnDiff     `json:"columns,omitempty"`
	Indexes     []IndexDiff      `json:"indexes,omitempty"`
	ForeignKeys []ForeignKeyDiff `json:"foreign_keys,omitempty"`
}

// ColumnDiff represents changes to a column within a table.
type ColumnDiff struct {
	ColumnName string       `json:"column_name"`
	Action     DiffAction   `json:"action"`
	Severity   DiffSeverity `json:"severity"`
	OldColumn  *Column      `json:"old_column,omitempty"` // For MODIFY
	NewColumn  *Column      `json:"new_column,omitempty"` // For ADD / MODIFY
	Changes    []string     `json:"changes,omitempty"`    // E.g., ["type changed from int(11) to varchar(255)"]
}

// IndexDiff represents changes to an index within a table.
type IndexDiff struct {
	IndexName string       `json:"index_name"`
	Action    DiffAction   `json:"action"`
	Severity  DiffSeverity `json:"severity"`
	OldIndex  *Index       `json:"old_index,omitempty"`
	NewIndex  *Index       `json:"new_index,omitempty"`
}

// ForeignKeyDiff represents changes to a foreign key within a table.
type ForeignKeyDiff struct {
	FKName string       `json:"fk_name"`
	Action DiffAction   `json:"action"`
	Severity DiffSeverity `json:"severity"`
	OldFK  *ForeignKey  `json:"old_fk,omitempty"`
	NewFK  *ForeignKey  `json:"new_fk,omitempty"`
}
