package differ

import (
	"strings"
	"testing"

	"github.com/bryanathallah/db-schema-differ/models"
)

func TestDiff_Identical(t *testing.T) {
	col := models.Column{
		Name:           "id",
		NormalizedType: "int",
		RawType:        "int(11)",
		Nullable:       "NO",
	}
	src := &models.Schema{
		Name: "src",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": col,
				},
				ColumnOrder: []string{"id"},
			},
		},
	}
	tgt := &models.Schema{
		Name: "tgt",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": col,
				},
				ColumnOrder: []string{"id"},
			},
		},
	}

	res, err := Diff(src, tgt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Tables) != 0 {
		t.Errorf("expected 0 differences, got %d", len(res.Tables))
	}
}

func TestDiff_AddTable(t *testing.T) {
	src := &models.Schema{
		Name: "src",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": {Name: "id", NormalizedType: "int"},
				},
				ColumnOrder: []string{"id"},
			},
		},
	}
	tgt := &models.Schema{
		Name:   "tgt",
		Tables: map[string]models.Table{},
	}

	res, err := Diff(src, tgt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Tables) != 1 {
		t.Fatalf("expected 1 table change, got %d", len(res.Tables))
	}
	if res.Tables[0].Action != models.ActionAdd {
		t.Errorf("expected ADD action, got %s", res.Tables[0].Action)
	}
	if res.Summary.TablesAdded != 1 {
		t.Errorf("expected Summary.TablesAdded to be 1, got %d", res.Summary.TablesAdded)
	}
}

func TestDiff_DropTable(t *testing.T) {
	src := &models.Schema{
		Name:   "src",
		Tables: map[string]models.Table{},
	}
	tgt := &models.Schema{
		Name: "tgt",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": {Name: "id", NormalizedType: "int"},
				},
				ColumnOrder: []string{"id"},
			},
		},
	}

	res, err := Diff(src, tgt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Tables) != 1 {
		t.Fatalf("expected 1 table change, got %d", len(res.Tables))
	}
	if res.Tables[0].Action != models.ActionDrop {
		t.Errorf("expected DROP action, got %s", res.Tables[0].Action)
	}
	if res.Tables[0].Severity != models.SeverityDanger {
		t.Errorf("expected SeverityDanger, got %s", res.Tables[0].Severity)
	}
	if res.Summary.TablesDropped != 1 {
		t.Errorf("expected Summary.TablesDropped to be 1, got %d", res.Summary.TablesDropped)
	}
}

func TestDiff_RenameHeuristic(t *testing.T) {
	src := &models.Schema{
		Name: "src",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"new_email": {Name: "new_email", NormalizedType: "varchar(255)"},
				},
				ColumnOrder: []string{"new_email"},
			},
		},
	}
	tgt := &models.Schema{
		Name: "tgt",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"old_email": {Name: "old_email", NormalizedType: "varchar(255)"},
				},
				ColumnOrder: []string{"old_email"},
			},
		},
	}

	res, err := Diff(src, tgt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Tables) != 1 {
		t.Fatalf("expected 1 table change, got %d", len(res.Tables))
	}

	cols := res.Tables[0].Columns
	// We expect 1 ADD with possible rename comment (the drop is absorbed by the rename heuristic matching)
	if len(cols) != 1 {
		t.Fatalf("expected 1 column change (rename match), got %d", len(cols))
	}

	foundRenameComment := false
	for _, c := range cols {
		for _, change := range c.Changes {
			if strings.Contains(change, "Possible rename from 'old_email' to 'new_email'") {
				foundRenameComment = true
			}
		}
	}

	if !foundRenameComment {
		t.Errorf("expected rename heuristic to detect potential rename comment")
	}
}
