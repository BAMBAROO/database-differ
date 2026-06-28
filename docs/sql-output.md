# SQL Generation and Ordering

`db-schema-differ` generates standard SQL DDL statements to bring target databases in sync with the desired source database schema.

---

## Three-Pass DDL Compilation

To handle circular references and avoid reference constraint violations during schema migrations, the tool implements a **Three-Pass compilation architecture**:

```
[Diff Result]
    │
    ├──► Pass 1: Structure (CREATE / DROP tables without foreign keys)
    │
    ├──► Pass 2: Columns (ADD / MODIFY / DROP columns & drop indexes)
    │
    └──► Pass 3: Constraints (ADD indexes & ADD / DROP foreign keys)
```

### Pass 1: Structure Pass
- **Actions**: Creates tables or drops tables.
- **FK Deferral**: No foreign key definitions are generated inside `CREATE TABLE` statements.
- **Topological Sorting**: Table creations are sorted topologically based on foreign key references. This means independent tables (e.g. parent tables) are created before dependent tables (e.g. child tables).
- **Circular References**: Since foreign keys are not created yet, circular references are safely ignored in this step and do not cause failures.

### Pass 2: Column Pass
- **Actions**: Add columns, modify column types, drop columns, and drop indexes.
- **Index Ordering**: Indexes are dropped before modifying any columns associated with those indexes to prevent index lock errors.
- **Positioning**: Columns are added with correct positional specifiers (MySQL: `FIRST` / `AFTER`).

### Pass 3: Constraint Pass
- **Actions**: Adds index definitions and adds/drops foreign key constraints.
- **Safety**: Since all tables and columns are guaranteed to exist after Pass 1 and Pass 2, foreign keys can be linked in any order, resolving circular references cleanly.

---

## Transactional Wrapping

- **PostgreSQL**: The entire generated SQL script is wrapped inside a transaction block (`BEGIN;` and `COMMIT;`). If any migration step fails, the entire run is rolled back.
- **MySQL**: The generated script is **not** wrapped in a transaction block. Instead, it relies on the state file to support resuming execution from the point of failure.

---

## Safe vs Breaking Migration Diffs

If you run the CLI with the `--safe-only` flag or generate migrations with `--safe-only`:
- All `DROP` statements (dropping tables, dropping columns, dropping indexes, dropping foreign keys) are omitted from the generated SQL.
- Only additions (`ADD`, `CREATE`) and safe modifications are performed.
- This is useful for blue-green deployments where the old application version must continue running against the database.
