package introspector

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/bryanathallah/db-schema-differ/models"
)

type MySQLIntrospector struct{}

func NewMySQLIntrospector() Introspector {
	return &MySQLIntrospector{}
}

func (mi *MySQLIntrospector) Introspect(db *sql.DB, dbName string) (*models.Schema, error) {
	if dbName == "" {
		err := db.QueryRow("SELECT DATABASE()").Scan(&dbName)
		if err != nil {
			return nil, fmt.Errorf("failed to detect current database name: %w", err)
		}
	}

	schema := &models.Schema{
		Name:   dbName,
		Tables: make(map[string]models.Table),
	}

	// 1. Fetch tables
	tablesQuery := `
		SELECT TABLE_NAME, ENGINE, TABLE_COLLATION, TABLE_COMMENT 
		FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'`
	rows, err := db.Query(tablesQuery, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, engine, collation, comment string
		if err := rows.Scan(&name, &engine, &collation, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		schema.Tables[name] = models.Table{
			Name:        name,
			Engine:      engine,
			Collation:   collation,
			Comment:     comment,
			Columns:     make(map[string]models.Column),
			ColumnOrder: []string{},
			Indexes:     make(map[string]models.Index),
			ForeignKeys: make(map[string]models.ForeignKey),
		}
	}

	// 2. Fetch columns
	columnsQuery := `
		SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COLLATION_NAME, COLUMN_COMMENT, ORDINAL_POSITION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`
	cRows, err := db.Query(columnsQuery, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer cRows.Close()

	for cRows.Next() {
		var tableName, columnName, dataType, columnType, isNullable, extra string
		var collation, comment, columnDefault sql.NullString
		var ordinalPosition int

		if err := cRows.Scan(&tableName, &columnName, &dataType, &columnType, &isNullable, &columnDefault, &extra, &collation, &comment, &ordinalPosition); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}

		table, ok := schema.Tables[tableName]
		if !ok {
			continue // Skip if table was filtered out (e.g. view)
		}

		var defVal *string
		if columnDefault.Valid {
			v := columnDefault.String
			defVal = &v
		}

		col := models.Column{
			Name:           columnName,
			NormalizedType: NormalizeMySQLType(dataType, columnType),
			RawType:        columnType,
			Nullable:       isNullable,
			DefaultValue:   defVal,
			AutoIncrement:  strings.Contains(strings.ToLower(extra), "auto_increment"),
			Collation:      collation.String,
			Comment:        comment.String,
			Position:       ordinalPosition,
		}

		table.Columns[columnName] = col
		table.ColumnOrder = append(table.ColumnOrder, columnName)
		schema.Tables[tableName] = table
	}

	// 3. Fetch indexes
	indexesQuery := `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX, INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`
	iRows, err := db.Query(indexesQuery, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer iRows.Close()

	type idxCol struct {
		columnName string
		seq        int
	}
	// Map to group composite indexes temporary: table -> indexName -> columns
	tempIndexes := make(map[string]map[string]struct {
		isUnique bool
		idxType  string
		cols     []idxCol
	})

	for iRows.Next() {
		var tableName, indexName, idxType, columnName string
		var nonUnique, seqInIndex int

		if err := iRows.Scan(&tableName, &indexName, &nonUnique, &columnName, &seqInIndex, &idxType); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		if _, ok := schema.Tables[tableName]; !ok {
			continue
		}

		if tempIndexes[tableName] == nil {
			tempIndexes[tableName] = make(map[string]struct {
				isUnique bool
				idxType  string
				cols     []idxCol
			})
		}

		entry := tempIndexes[tableName][indexName]
		entry.isUnique = (nonUnique == 0)
		entry.idxType = idxType
		entry.cols = append(entry.cols, idxCol{columnName: columnName, seq: seqInIndex})
		tempIndexes[tableName][indexName] = entry
	}

	for tableName, idxs := range tempIndexes {
		table := schema.Tables[tableName]
		for indexName, entry := range idxs {
			// Sort columns by seq
			columns := make([]string, len(entry.cols))
			for _, col := range entry.cols {
				if col.seq <= len(columns) && col.seq > 0 {
					columns[col.seq-1] = col.columnName
				}
			}

			// Determine index type
			var t string
			if indexName == "PRIMARY" {
				t = "PRIMARY"
			} else if entry.idxType == "FULLTEXT" {
				t = "FULLTEXT"
			} else if entry.isUnique {
				t = "UNIQUE"
			} else {
				t = "INDEX"
			}

			table.Indexes[indexName] = models.Index{
				Name:    indexName,
				Type:    t,
				Columns: columns,
			}
		}
		schema.Tables[tableName] = table
	}

	// 4. Fetch foreign keys
	fkQuery := `
		SELECT 
			k.CONSTRAINT_NAME, 
			k.TABLE_NAME, 
			k.COLUMN_NAME, 
			k.REFERENCED_TABLE_NAME, 
			k.REFERENCED_COLUMN_NAME,
			r.DELETE_RULE,
			r.UPDATE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS r 
		  ON k.CONSTRAINT_NAME = r.CONSTRAINT_NAME AND k.CONSTRAINT_SCHEMA = r.CONSTRAINT_SCHEMA
		WHERE k.CONSTRAINT_SCHEMA = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL`
	fkRows, err := db.Query(fkQuery, dbName)
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

var mysqlIntWidthRegex = regexp.MustCompile(`^(int|bigint|mediumint|smallint|tinyint)\(\d+\)(.*)$`)

// NormalizeMySQLType formats column types to canonical, normalized forms.
func NormalizeMySQLType(dataType, columnType string) string {
	dataType = strings.ToLower(dataType)
	columnType = strings.ToLower(columnType)

	// Keep tinyint(1) since it's the standard boolean in MySQL
	if strings.HasPrefix(columnType, "tinyint(1)") {
		return "tinyint(1)"
	}

	// Strip display width from integers (e.g. int(11) -> int, bigint(20) -> bigint)
	if mysqlIntWidthRegex.MatchString(columnType) {
		columnType = mysqlIntWidthRegex.ReplaceAllString(columnType, "$1$2")
	}

	// Alias mappings
	switch dataType {
	case "integer":
		columnType = strings.Replace(columnType, "integer", "int", 1)
	case "bool", "boolean":
		return "tinyint(1)"
	}

	return strings.TrimSpace(columnType)
}
