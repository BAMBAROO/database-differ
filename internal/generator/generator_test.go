package generator

import (
	"strings"
	"testing"

	"github.com/bryanathallah/db-schema-differ/models"
)

func TestMySQLGenerator_ThreePass(t *testing.T) {
	// Setup a diff: table users added, table orders added which references users.
	src := &models.Schema{
		Name: "src",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": {Name: "id", RawType: "int", Nullable: "NO"},
				},
				ColumnOrder: []string{"id"},
				Indexes: map[string]models.Index{
					"PRIMARY": {Name: "PRIMARY", Type: "PRIMARY", Columns: []string{"id"}},
				},
			},
			"orders": {
				Name: "orders",
				Columns: map[string]models.Column{
					"id":      {Name: "id", RawType: "int", Nullable: "NO"},
					"user_id": {Name: "user_id", RawType: "int", Nullable: "NO"},
				},
				ColumnOrder: []string{"id", "user_id"},
				Indexes: map[string]models.Index{
					"PRIMARY": {Name: "PRIMARY", Type: "PRIMARY", Columns: []string{"id"}},
				},
				ForeignKeys: map[string]models.ForeignKey{
					"fk_orders_user": {
						Name:             "fk_orders_user",
						Column:           "user_id",
						ReferencedTable:  "users",
						ReferencedColumn: "id",
					},
				},
			},
		},
	}

	diff := &models.SchemaDiff{
		Tables: []models.TableDiff{
			{TableName: "users", Action: models.ActionAdd, Severity: models.SeveritySafe},
			{TableName: "orders", Action: models.ActionAdd, Severity: models.SeveritySafe},
		},
	}

	g := NewMySQLGenerator()
	stmts, err := g.Generate(diff, src, GenOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that users table is created BEFORE orders table (topological sorting)
	usersIdx, ordersIdx := -1, -1
	fkIdx := -1
	for i, s := range stmts {
		if strings.Contains(s, "CREATE TABLE `users`") {
			usersIdx = i
		}
		if strings.Contains(s, "CREATE TABLE `orders`") {
			ordersIdx = i
		}
		if strings.Contains(s, "ADD CONSTRAINT `fk_orders_user` FOREIGN KEY") {
			fkIdx = i
		}
	}

	if usersIdx == -1 || ordersIdx == -1 {
		t.Fatalf("could not find CREATE TABLE statements in generated output")
	}

	if usersIdx > ordersIdx {
		t.Errorf("expected 'users' table to be created before 'orders' table")
	}

	// In three-pass architecture, CREATE TABLE statements should NOT contain foreign keys,
	// and the foreign keys should be added in Pass 3.
	for _, s := range stmts[usersIdx:ordersIdx+1] {
		if strings.Contains(s, "FOREIGN KEY") {
			t.Errorf("CREATE TABLE statements should not contain FOREIGN KEY constraints in Pass 1")
		}
	}

	if fkIdx == -1 {
		t.Errorf("expected FOREIGN KEY constraint to be added separately in Pass 3")
	}
	if fkIdx < ordersIdx {
		t.Errorf("expected foreign key alteration to run after table creation")
	}
}

func TestPostgresGenerator_Transaction(t *testing.T) {
	src := &models.Schema{
		Name: "src",
		Tables: map[string]models.Table{
			"users": {
				Name: "users",
				Columns: map[string]models.Column{
					"id": {Name: "id", RawType: "int", Nullable: "NO"},
				},
				ColumnOrder: []string{"id"},
			},
		},
	}

	diff := &models.SchemaDiff{
		Tables: []models.TableDiff{
			{TableName: "users", Action: models.ActionAdd, Severity: models.SeveritySafe},
		},
	}

	g := NewPostgresGenerator()
	stmts, err := g.Generate(diff, src, GenOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmts) < 2 {
		t.Fatalf("expected transaction wrapping")
	}

	if stmts[0] != "BEGIN;" {
		t.Errorf("expected first statement to be 'BEGIN;', got %s", stmts[0])
	}
	if stmts[len(stmts)-1] != "COMMIT;" {
		t.Errorf("expected last statement to be 'COMMIT;', got %s", stmts[len(stmts)-1])
	}
}
