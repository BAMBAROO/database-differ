package reporter

import (
	"fmt"
	"io"

	"github.com/bryanathallah/db-schema-differ/models"
	"github.com/fatih/color"
)

type TerminalReporter struct {
	NoColor bool
}

func NewTerminalReporter(noColor bool) Reporter {
	return &TerminalReporter{NoColor: noColor}
}

func (r *TerminalReporter) Report(diff *models.SchemaDiff, w io.Writer) error {
	// Enable/disable colors
	if r.NoColor {
		color.NoColor = true
	}

	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	// Print Summary
	fmt.Fprintln(w, bold("Schema Diff Summary"))
	fmt.Fprintln(w, bold("───────────────────"))
	fmt.Fprintf(w, "%-9s: %s\n", "Source", cyan(diff.Summary.SourceDB))
	fmt.Fprintf(w, "%-9s: %s\n", "Target", cyan(diff.Summary.TargetDB))
	fmt.Fprintln(w)

	fmt.Fprintf(w, "%-9s: %s   %s   %s\n", "Tables",
		green(fmt.Sprintf("+%d added", diff.Summary.TablesAdded)),
		yellow(fmt.Sprintf("~%d modified", diff.Summary.TablesModified)),
		red(fmt.Sprintf("-%d removed", diff.Summary.TablesDropped)),
	)
	fmt.Fprintf(w, "%-9s: %s   %s   %s\n", "Columns",
		green(fmt.Sprintf("+%d added", diff.Summary.ColumnsAdded)),
		yellow(fmt.Sprintf("~%d modified", diff.Summary.ColumnsModified)),
		red(fmt.Sprintf("-%d removed", diff.Summary.ColumnsDropped)),
	)
	fmt.Fprintf(w, "%-9s: %s   %s\n", "Indexes",
		green(fmt.Sprintf("+%d added", diff.Summary.IndexesAdded)),
		red(fmt.Sprintf("-%d removed", diff.Summary.IndexesDropped)),
	)
	fmt.Fprintf(w, "%-9s: %s   %s\n", "FKs",
		green(fmt.Sprintf("+%d added", diff.Summary.FKsAdded)),
		red(fmt.Sprintf("-%d removed", diff.Summary.FKsDropped)),
	)
	fmt.Fprintln(w)

	// Detail Print
	if len(diff.Tables) == 0 {
		fmt.Fprintln(w, green("Schemas are identical! No changes detected."))
		return nil
	}

	fmt.Fprintln(w, bold("Differences Details"))
	fmt.Fprintln(w, bold("───────────────────"))

	for _, tDiff := range diff.Tables {
		severityBadge := formatSeverity(tDiff.Severity, r.NoColor)

		switch tDiff.Action {
		case models.ActionAdd:
			fmt.Fprintf(w, "%s %s %s\n", green("+"), bold(tDiff.TableName), severityBadge)
			fmt.Fprintf(w, "    %s\n\n", color.New(color.Faint).Sprintf("Table needs to be created"))

		case models.ActionDrop:
			fmt.Fprintf(w, "%s %s %s\n", red("-"), bold(tDiff.TableName), severityBadge)
			fmt.Fprintf(w, "    %s\n\n", color.New(color.Faint).Sprintf("Table will be dropped"))

		case models.ActionModify:
			fmt.Fprintf(w, "%s %s %s\n", yellow("~"), bold(tDiff.TableName), severityBadge)
			if tDiff.Comment != "" {
				fmt.Fprintf(w, "    %s\n", color.New(color.Faint).Sprintf("(%s)", tDiff.Comment))
			}

			// Column modifications
			for _, col := range tDiff.Columns {
				colSev := formatSeverity(col.Severity, r.NoColor)
				switch col.Action {
				case models.ActionAdd:
					fmt.Fprintf(w, "    %s %s %s (type: %s)\n", green("+"), col.ColumnName, colSev, col.NewColumn.RawType)
					for _, change := range col.Changes {
						fmt.Fprintf(w, "        %s\n", color.New(color.Faint).Sprintf("%s", change))
					}
				case models.ActionDrop:
					fmt.Fprintf(w, "    %s %s %s (type: %s)\n", red("-"), col.ColumnName, colSev, col.OldColumn.RawType)
				case models.ActionModify:
					fmt.Fprintf(w, "    %s %s %s\n", yellow("~"), col.ColumnName, colSev)
					for _, change := range col.Changes {
						fmt.Fprintf(w, "        %s\n", change)
					}
				}
			}

			// Index modifications
			for _, idx := range tDiff.Indexes {
				idxSev := formatSeverity(idx.Severity, r.NoColor)
				switch idx.Action {
				case models.ActionAdd:
					cols := idx.NewIndex.Columns
					fmt.Fprintf(w, "    %s Index %s %s (%s: %s)\n", green("+"), idx.IndexName, idxSev, idx.NewIndex.Type, fmt.Sprintf("%v", cols))
				case models.ActionDrop:
					cols := idx.OldIndex.Columns
					fmt.Fprintf(w, "    %s Index %s %s (%s: %s)\n", red("-"), idx.IndexName, idxSev, idx.OldIndex.Type, fmt.Sprintf("%v", cols))
				}
			}

			// ForeignKey modifications
			for _, fk := range tDiff.ForeignKeys {
				fkSev := formatSeverity(fk.Severity, r.NoColor)
				switch fk.Action {
				case models.ActionAdd:
					newFK := fk.NewFK
					fmt.Fprintf(w, "    %s FK %s %s (%s -> %s.%s)\n", green("+"), fk.FKName, fkSev, newFK.Column, newFK.ReferencedTable, newFK.ReferencedColumn)
				case models.ActionDrop:
					oldFK := fk.OldFK
					fmt.Fprintf(w, "    %s FK %s %s (%s -> %s.%s)\n", red("-"), fk.FKName, fkSev, oldFK.Column, oldFK.ReferencedTable, oldFK.ReferencedColumn)
				}
			}
			fmt.Fprintln(w)
		}
	}

	// Print Risk Alert if Warnings or Dangers present
	if diff.Summary.WarningsCount > 0 || diff.Summary.DangersCount > 0 {
		fmt.Fprintln(w, bold("Risk Assessment"))
		fmt.Fprintln(w, bold("───────────────"))
		if diff.Summary.DangersCount > 0 {
			fmt.Fprintln(w, red(fmt.Sprintf("⚠️  DANGER: %d destructive actions detected (e.g. DROP operations). Verify details before executing.", diff.Summary.DangersCount)))
		}
		if diff.Summary.WarningsCount > 0 {
			fmt.Fprintln(w, yellow(fmt.Sprintf("⚠️  WARNING: %d actions detected that may cause data loss or truncation. Proceed with caution.", diff.Summary.WarningsCount)))
		}
		fmt.Fprintln(w)
	}

	return nil
}

func formatSeverity(s models.DiffSeverity, noColor bool) string {
	if noColor {
		return fmt.Sprintf("[%s]", s)
	}
	switch s {
	case models.SeveritySafe:
		return color.New(color.FgGreen).Sprintf("[SAFE]")
	case models.SeverityWarning:
		return color.New(color.FgYellow).Sprintf("[WARNING]")
	case models.SeverityDanger:
		return color.New(color.FgRed).Sprintf("[DANGER]")
	default:
		return ""
	}
}
