package differ

import (
	"fmt"
	"strings"

	"github.com/bryanathallah/db-schema-differ/models"
)

// Diff calculates the differences between source schema (desired state) and target schema (current state).
func Diff(source, target *models.Schema) (*models.SchemaDiff, error) {
	diff := &models.SchemaDiff{
		Summary: models.SchemaDiffSummary{
			SourceDB: source.Name,
			TargetDB: target.Name,
		},
		Tables: []models.TableDiff{},
	}

	// 1. Detect added tables (in Source, not in Target)
	for sTableName, sTable := range source.Tables {
		tTable, exists := target.Tables[sTableName]
		if !exists {
			tDiff := models.TableDiff{
				TableName: sTableName,
				Action:    models.ActionAdd,
				Severity:  models.SeveritySafe,
				Comment:   "Table added",
			}
			diff.Tables = append(diff.Tables, tDiff)
			diff.Summary.TablesAdded++
			diff.Summary.ColumnsAdded += len(sTable.Columns)
			diff.Summary.IndexesAdded += len(sTable.Indexes)
			diff.Summary.FKsAdded += len(sTable.ForeignKeys)
			continue
		}

		// Table exists in both, calculate changes within the table
		tDiff := diffTable(sTable, tTable)
		if tDiff.Action == models.ActionModify {
			diff.Tables = append(diff.Tables, tDiff)
			diff.Summary.TablesModified++

			// Add inner item statistics
			for _, col := range tDiff.Columns {
				if col.Action == models.ActionAdd {
					diff.Summary.ColumnsAdded++
				} else if col.Action == models.ActionModify {
					diff.Summary.ColumnsModified++
				} else if col.Action == models.ActionDrop {
					diff.Summary.ColumnsDropped++
				}
				if col.Severity == models.SeverityWarning {
					diff.Summary.WarningsCount++
				} else if col.Severity == models.SeverityDanger {
					diff.Summary.DangersCount++
				}
			}
			for _, idx := range tDiff.Indexes {
				if idx.Action == models.ActionAdd {
					diff.Summary.IndexesAdded++
				} else if idx.Action == models.ActionDrop {
					diff.Summary.IndexesDropped++
				}
				if idx.Severity == models.SeverityWarning {
					diff.Summary.WarningsCount++
				} else if idx.Severity == models.SeverityDanger {
					diff.Summary.DangersCount++
				}
			}
			for _, fk := range tDiff.ForeignKeys {
				if fk.Action == models.ActionAdd {
					diff.Summary.FKsAdded++
				} else if fk.Action == models.ActionDrop {
					diff.Summary.FKsDropped++
				}
				if fk.Severity == models.SeverityWarning {
					diff.Summary.WarningsCount++
				} else if fk.Severity == models.SeverityDanger {
					diff.Summary.DangersCount++
				}
			}
		}
	}

	// 2. Detect dropped tables (in Target, not in Source)
	for tTableName, tTable := range target.Tables {
		_, exists := source.Tables[tTableName]
		if !exists {
			tDiff := models.TableDiff{
				TableName: tTableName,
				Action:    models.ActionDrop,
				Severity:  models.SeverityDanger,
				Comment:   "Table dropped",
			}
			diff.Tables = append(diff.Tables, tDiff)
			diff.Summary.TablesDropped++
			diff.Summary.ColumnsDropped += len(tTable.Columns)
			diff.Summary.IndexesDropped += len(tTable.Indexes)
			diff.Summary.FKsDropped += len(tTable.ForeignKeys)
			diff.Summary.DangersCount++
		}
	}

	return diff, nil
}

// diffTable compares two tables with the same name.
func diffTable(source, target models.Table) models.TableDiff {
	tDiff := models.TableDiff{
		TableName:   source.Name,
		Action:      models.ActionModify,
		Severity:    models.SeveritySafe,
		Columns:     []models.ColumnDiff{},
		Indexes:     []models.IndexDiff{},
		ForeignKeys: []models.ForeignKeyDiff{},
	}

	modified := false

	// Compare Engine, Collation, Comments (primarily for MySQL/PostgreSQL metadata updates)
	if source.Engine != target.Engine && source.Engine != "" && target.Engine != "" {
		tDiff.Comment = fmt.Sprintf("Engine changed from %s to %s. ", target.Engine, source.Engine)
		tDiff.Severity = models.SeverityWarning
		modified = true
	}
	if source.Collation != target.Collation && source.Collation != "" && target.Collation != "" {
		tDiff.Comment += fmt.Sprintf("Collation changed from %s to %s. ", target.Collation, source.Collation)
		modified = true
	}

	// 1. Column changes
	addedCols := []models.Column{}
	droppedCols := []models.Column{}

	// Added columns & Modified columns
	for sColName, sCol := range source.Columns {
		tCol, exists := target.Columns[sColName]
		if !exists {
			addedCols = append(addedCols, sCol)
			continue
		}

		cDiff := diffColumn(sCol, tCol)
		if cDiff.Action == models.ActionModify {
			tDiff.Columns = append(tDiff.Columns, cDiff)
			modified = true
		}
	}

	// Dropped columns
	for tColName, tCol := range target.Columns {
		_, exists := source.Columns[tColName]
		if !exists {
			droppedCols = append(droppedCols, tCol)
		}
	}

	// Heuristic rename detection: match added and dropped columns by type
	matchedDrops := make(map[string]bool)
	matchedAdds := make(map[string]bool)

	for _, sCol := range addedCols {
		for _, tCol := range droppedCols {
			if matchedDrops[tCol.Name] || matchedAdds[sCol.Name] {
				continue
			}

			// If type matches and they are within the same table, it's a likely rename
			if sCol.NormalizedType == tCol.NormalizedType {
				cDiff := models.ColumnDiff{
					ColumnName: sCol.Name,
					Action:     models.ActionAdd,
					Severity:   models.SeveritySafe,
					NewColumn:  &sCol,
					Changes:    []string{fmt.Sprintf("Possible rename from '%s' to '%s'", tCol.Name, sCol.Name)},
				}
				tDiff.Columns = append(tDiff.Columns, cDiff)
				matchedAdds[sCol.Name] = true
				matchedDrops[tCol.Name] = true
				modified = true
				break
			}
		}
	}

	// Add remaining unmatched added columns
	for _, sCol := range addedCols {
		if matchedAdds[sCol.Name] {
			continue
		}
		cDiff := models.ColumnDiff{
			ColumnName: sCol.Name,
			Action:     models.ActionAdd,
			Severity:   models.SeveritySafe,
			NewColumn:  &sCol,
		}
		tDiff.Columns = append(tDiff.Columns, cDiff)
		modified = true
	}

	// Add remaining unmatched dropped columns
	for _, tCol := range droppedCols {
		if matchedDrops[tCol.Name] {
			continue
		}
		cDiff := models.ColumnDiff{
			ColumnName: tCol.Name,
			Action:     models.ActionDrop,
			Severity:   models.SeverityDanger,
			OldColumn:  &tCol,
		}
		tDiff.Columns = append(tDiff.Columns, cDiff)
		modified = true
	}

	// 2. Index changes
	// Added & Modified indexes
	for sIdxName, sIdx := range source.Indexes {
		tIdx, exists := target.Indexes[sIdxName]
		if !exists {
			tDiff.Indexes = append(tDiff.Indexes, models.IndexDiff{
				IndexName: sIdxName,
				Action:    models.ActionAdd,
				Severity:  models.SeveritySafe,
				NewIndex:  &sIdx,
			})
			modified = true
			continue
		}

		if diffIndex(sIdx, tIdx) {
			// If index changed, drop old and add new
			tDiff.Indexes = append(tDiff.Indexes, models.IndexDiff{
				IndexName: sIdxName,
				Action:    models.ActionDrop,
				Severity:  models.SeverityDanger,
				OldIndex:  &tIdx,
			})
			tDiff.Indexes = append(tDiff.Indexes, models.IndexDiff{
				IndexName: sIdxName,
				Action:    models.ActionAdd,
				Severity:  models.SeveritySafe,
				NewIndex:  &sIdx,
			})
			modified = true
		}
	}

	// Dropped indexes
	for tIdxName, tIdx := range target.Indexes {
		_, exists := source.Indexes[tIdxName]
		if !exists {
			tDiff.Indexes = append(tDiff.Indexes, models.IndexDiff{
				IndexName: tIdxName,
				Action:    models.ActionDrop,
				Severity:  models.SeverityDanger,
				OldIndex:  &tIdx,
			})
			modified = true
		}
	}

	// 3. Foreign Key changes
	// Added & Modified FKs
	for sFKName, sFK := range source.ForeignKeys {
		tFK, exists := target.ForeignKeys[sFKName]
		if !exists {
			tDiff.ForeignKeys = append(tDiff.ForeignKeys, models.ForeignKeyDiff{
				FKName:   sFKName,
				Action:   models.ActionAdd,
				Severity: models.SeveritySafe,
				NewFK:    &sFK,
			})
			modified = true
			continue
		}

		if diffFK(sFK, tFK) {
			// If FK changed, drop old and add new
			tDiff.ForeignKeys = append(tDiff.ForeignKeys, models.ForeignKeyDiff{
				FKName:   sFKName,
				Action:   models.ActionDrop,
				Severity: models.SeverityDanger,
				OldFK:    &tFK,
			})
			tDiff.ForeignKeys = append(tDiff.ForeignKeys, models.ForeignKeyDiff{
				FKName:   sFKName,
				Action:   models.ActionAdd,
				Severity: models.SeveritySafe,
				NewFK:    &sFK,
			})
			modified = true
		}
	}

	// Dropped FKs
	for tFKName, tFK := range target.ForeignKeys {
		_, exists := source.ForeignKeys[tFKName]
		if !exists {
			tDiff.ForeignKeys = append(tDiff.ForeignKeys, models.ForeignKeyDiff{
				FKName:   tFKName,
				Action:   models.ActionDrop,
				Severity: models.SeverityDanger,
				OldFK:    &tFK,
			})
			modified = true
		}
	}

	if modified {
		tDiff.Comment = strings.TrimSpace(tDiff.Comment)
		return tDiff
	}

	return models.TableDiff{TableName: source.Name, Action: ""}
}

// diffColumn detects differences between two column definitions.
func diffColumn(source, target models.Column) models.ColumnDiff {
	cDiff := models.ColumnDiff{
		ColumnName: source.Name,
		Action:     models.ActionModify,
		Severity:   models.SeveritySafe,
		OldColumn:  &target,
		NewColumn:  &source,
		Changes:    []string{},
	}

	changed := false

	// Compare normalized types
	if source.NormalizedType != target.NormalizedType {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("type changed from '%s' to '%s'", target.NormalizedType, source.NormalizedType))
		cDiff.Severity = models.SeverityWarning
		changed = true
	}

	// Compare nullability
	if source.Nullable != target.Nullable {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("nullable changed from '%s' to '%s'", target.Nullable, source.Nullable))
		// If column is changing to NOT NULL and it was NULL, it's a potential risk if table has data.
		if source.Nullable == "NO" {
			cDiff.Severity = models.SeverityWarning
		}
		changed = true
	}

	// Compare Default Values
	sDef := getDefStr(source.DefaultValue)
	tDef := getDefStr(target.DefaultValue)
	if sDef != tDef {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("default changed from %s to %s", tDef, sDef))
		changed = true
	}

	// Compare AutoIncrement
	if source.AutoIncrement != target.AutoIncrement {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("auto_increment changed from %v to %v", target.AutoIncrement, source.AutoIncrement))
		cDiff.Severity = models.SeverityWarning
		changed = true
	}

	// Compare Collation
	if source.Collation != target.Collation && source.Collation != "" && target.Collation != "" {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("collation changed from '%s' to '%s'", target.Collation, source.Collation))
		changed = true
	}

	// Compare comments
	if source.Comment != target.Comment {
		cDiff.Changes = append(cDiff.Changes, "comment changed")
		changed = true
	}

	// Compare column order position
	if source.Position != target.Position {
		cDiff.Changes = append(cDiff.Changes, fmt.Sprintf("position changed from %d to %d", target.Position, source.Position))
		changed = true
	}

	if changed {
		return cDiff
	}

	return models.ColumnDiff{ColumnName: source.Name, Action: ""}
}

func getDefStr(def *string) string {
	if def == nil {
		return "NULL"
	}
	return fmt.Sprintf("'%s'", *def)
}

// diffIndex returns true if the index has changed.
func diffIndex(source, target models.Index) bool {
	if source.Type != target.Type {
		return true
	}
	if len(source.Columns) != len(target.Columns) {
		return true
	}
	for i, c := range source.Columns {
		if c != target.Columns[i] {
			return true
		}
	}
	return false
}

// diffFK returns true if the foreign key has changed.
func diffFK(source, target models.ForeignKey) bool {
	return source.Column != target.Column ||
		source.ReferencedTable != target.ReferencedTable ||
		source.ReferencedColumn != target.ReferencedColumn ||
		strings.ToLower(source.OnDelete) != strings.ToLower(target.OnDelete) ||
		strings.ToLower(source.OnUpdate) != strings.ToLower(target.OnUpdate)
}
