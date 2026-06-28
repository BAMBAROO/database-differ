package introspector

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/bryanathallah/db-schema-differ/models"
)

type PostgresIntrospector struct{}

func NewPostgresIntrospector() Introspector {
	return &PostgresIntrospector{}
}

func (pi *PostgresIntrospector) Introspect(db *sql.DB, schemaName string) (*models.Schema, error) {
	if schemaName == "" {
		schemaName = "public" // Default schema in PG
	}

	schema := &models.Schema{
		Name:   schemaName,
		Tables: make(map[string]models.Table),
	}

	// 1. Fetch table names and comments
	tablesQuery := `
		SELECT c.relname AS table_name, COALESCE(d.description, '') AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_description d ON d.objoid = c.oid AND d.objsubid = 0
		WHERE n.nspname = $1 AND c.relkind = 'r'
		ORDER BY table_name`
	rows, err := db.Query(tablesQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		schema.Tables[name] = models.Table{
			Name:        name,
			Comment:     comment,
			Columns:     make(map[string]models.Column),
			ColumnOrder: []string{},
			Indexes:     make(map[string]models.Index),
			ForeignKeys: make(map[string]models.ForeignKey),
		}
	}

	// 2. Fetch column comments in a map
	columnComments := make(map[string]map[string]string) // table -> column -> comment
	commentsQuery := `
		SELECT 
			c.relname AS table_name,
			a.attname AS column_name,
			COALESCE(d.description, '') AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid
		LEFT JOIN pg_description d ON d.objoid = c.oid AND d.objsubid = a.attnum
		WHERE n.nspname = $1 AND c.relkind = 'r' AND a.attnum > 0 AND NOT a.attisdropped`
	commRows, err := db.Query(commentsQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query column comments: %w", err)
	}
	defer commRows.Close()

	for commRows.Next() {
		var tName, cName, comment string
		if err := commRows.Scan(&tName, &cName, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan column comment: %w", err)
		}
		if columnComments[tName] == nil {
			columnComments[tName] = make(map[string]string)
		}
		columnComments[tName][cName] = comment
	}

	// 3. Fetch columns
	columnsQuery := `
		SELECT 
			table_name, 
			column_name, 
			data_type, 
			is_nullable, 
			column_default, 
			character_maximum_length,
			numeric_precision,
			numeric_scale,
			collation_name,
			ordinal_position
		FROM information_schema.columns
		WHERE table_schema = $1
		ORDER BY table_name, ordinal_position`
	cRows, err := db.Query(columnsQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer cRows.Close()

	for cRows.Next() {
		var tableName, columnName, dataType, isNullable string
		var columnDefault sql.NullString
		var charLength, numPrecision, numScale sql.NullInt64
		var collationName sql.NullString
		var ordinalPosition int

		err := cRows.Scan(
			&tableName, &columnName, &dataType, &isNullable, &columnDefault,
			&charLength, &numPrecision, &numScale, &collationName, &ordinalPosition,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}

		table, ok := schema.Tables[tableName]
		if !ok {
			continue
		}

		var defVal *string
		if columnDefault.Valid {
			v := columnDefault.String
			defVal = &v
		}

		normalizedType := NormalizePostgresType(dataType, charLength, numPrecision, numScale)

		// Get comment from our pre-fetched map
		comment := ""
		if cols, ok := columnComments[tableName]; ok {
			comment = cols[columnName]
		}

		col := models.Column{
			Name:           columnName,
			NormalizedType: normalizedType,
			RawType:        dataType,
			Nullable:       isNullable,
			DefaultValue:   defVal,
			AutoIncrement:  defVal != nil && strings.Contains(strings.ToLower(*defVal), "nextval("),
			Collation:      collationName.String,
			Comment:        comment,
			Position:       ordinalPosition,
		}

		table.Columns[columnName] = col
		table.ColumnOrder = append(table.ColumnOrder, columnName)
		schema.Tables[tableName] = table
	}

	// 4. Fetch indexes
	// Using pg_catalog tables to inspect indexes
	indexesQuery := `
		SELECT
			t.relname AS table_name,
			i.relname AS index_name,
			ix.indisprimary AS is_primary,
			ix.indisunique AS is_unique,
			ARRAY_TO_STRING(ARRAY(
				SELECT pg_get_indexdef(ix.indexrelid, k + 1, true)
				FROM generate_subscripts(ix.indkey, 1) AS k
				ORDER BY k
			), ',') AS index_columns
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1
		ORDER BY table_name, index_name`
	iRows, err := db.Query(indexesQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer iRows.Close()

	for iRows.Next() {
		var tableName, indexName, indexColumns string
		var isPrimary, isUnique bool

		if err := iRows.Scan(&tableName, &indexName, &isPrimary, &isUnique, &indexColumns); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		table, ok := schema.Tables[tableName]
		if !ok {
			continue
		}

		// Split columns by comma
		cols := strings.Split(indexColumns, ",")
		for i, c := range cols {
			cols[i] = strings.TrimSpace(c)
		}

		var t string
		if isPrimary {
			t = "PRIMARY"
		} else if isUnique {
			t = "UNIQUE"
		} else {
			t = "INDEX"
		}

		table.Indexes[indexName] = models.Index{
			Name:    indexName,
			Type:    t,
			Columns: cols,
		}
		schema.Tables[tableName] = table
	}

	// 5. Fetch foreign keys
	// Note: We query standard information_schema tables
	fkQuery := `
		SELECT
			tc.constraint_name, 
			tc.table_name, 
			kcu.column_name, 
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints AS tc 
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		JOIN information_schema.referential_constraints AS rc
		  ON tc.constraint_name = rc.constraint_name
		  AND tc.table_schema = rc.constraint_schema
		JOIN information_schema.constraint_column_usage AS ccu
		  ON rc.unique_constraint_name = ccu.constraint_name
		  AND rc.unique_constraint_schema = ccu.constraint_schema
		WHERE tc.table_schema = $1 AND tc.constraint_type = 'FOREIGN KEY'
		ORDER BY tc.table_name, tc.constraint_name`
	fkRows, err := db.Query(fkQuery, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer fkRows.Close()

	for fkRows.Next() {
		var constraintName, tableName, columnName, refTableName, refColName, deleteRule, updateRule string
		if err := fkRows.Scan(&constraintName, &tableName, &columnName, &refTableName, &refColName, &deleteRule, &updateRule); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key row: %w", err)
		}

		table, ok := schema.Tables[tableName]
		if !ok {
			continue
		}

		table.ForeignKeys[constraintName] = models.ForeignKey{
			Name:             constraintName,
			Column:           columnName,
			ReferencedTable:  refTableName,
			ReferencedColumn: refColName,
			OnDelete:         deleteRule,
			OnUpdate:         updateRule,
		}
		schema.Tables[tableName] = table
	}

	return schema, nil
}

// NormalizePostgresType normalizes PostgreSQL types to standard forms.
func NormalizePostgresType(dataType string, charLength sql.NullInt64, numPrecision sql.NullInt64, numScale sql.NullInt64) string {
	dataType = strings.ToLower(dataType)

	switch dataType {
	case "integer":
		return "int"
	case "character varying":
		if charLength.Valid {
			return fmt.Sprintf("varchar(%d)", charLength.Int64)
		}
		return "varchar"
	case "character":
		if charLength.Valid {
			return fmt.Sprintf("char(%d)", charLength.Int64)
		}
		return "char"
	case "double precision":
		return "float8"
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "numeric":
		if numPrecision.Valid && numScale.Valid && numScale.Int64 > 0 {
			return fmt.Sprintf("numeric(%d,%d)", numPrecision.Int64, numScale.Int64)
		} else if numPrecision.Valid {
			return fmt.Sprintf("numeric(%d)", numPrecision.Int64)
		}
		return "numeric"
	}

	return dataType
}
