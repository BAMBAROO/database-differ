# DB Schema Differ & Migrator (Go CLI Tool)

`db-schema-differ` is a lightweight, dependency-free command-line tool written in Go to introspect, compare, and synchronize database schemas between two MySQL or PostgreSQL instances. It is designed for developers and CI/CD pipelines to preview schema changes, generate migrations, and safely apply them.

---

## Key Features

- **Multi-Dialect Support**: Full native support for **MySQL (8.0)** and **PostgreSQL (15+)** schemas.
- **Three-Pass SQL Generation**: Intelligently handles circular foreign key dependencies by separating structural changes, column modifications, and constraints creation.
- **PostgreSQL Transactional Safety**: Entire PostgreSQL migration run executes inside a single database transaction block. Any failure triggers a clean rollback.
- **MySQL State File & `--resume` support**: Since MySQL doesn't support transactional DDL, the tool writes a `migration_state.json` tracker file. If a migration fails mid-way, you can edit the database or schema and run `--resume` to continue exactly from the failed step.
- **Heuristic Rename Detection**: Detects column renames by matching added/dropped column types within the same table.
- **Rich Output Formats**: Supports colorized terminal tables, JSON, and static self-contained HTML reports with filters.
- **Zero External Dependencies**: Compiles to a single binary with `CGO_ENABLED=0` for cross-platform compatibility (macOS, Windows, Linux).

---

## Quick Start

### 1. Download Pre-built Binaries
You can download the compiled standalone executables for your operating system directly:
- **[Latest GitHub Releases (All Platforms)](https://github.com/BAMBAROO/database-differ/releases)**

#### Running on macOS / Linux
1. Open terminal and navigate to the directory where you downloaded the file.
2. Grant executable permission:
   ```bash
   chmod +x db-schema-differ
   ```
3. Run the application:
   ```bash
   ./db-schema-differ
   ```
   *(On macOS, if blocked by Gatekeeper, go to **System Settings > Privacy & Security** and click **Open Anyway**).*

#### Running on Windows
- Simply **double-click** `db-schema-differ.exe` to launch the server, or open **Command Prompt/PowerShell** and run:
  ```powershell
  .\db-schema-differ.exe
  ```

---

### 2. How to Use the Web UI (Default Mode)
When you run the binary with no arguments:
```bash
./db-schema-differ
```
It starts the premium local Web UI Server on `http://127.0.0.1:8080` and **automatically opens your web browser**. 

Within the dashboard, you can:
- Input connection DSNs (or load default `.env` variables).
- **Test Connection** with single-click catalog verification.
- Review a **Visual Comparison** of schema changes (Safe, Warning, and Destructive actions).
- Switch between **Comparison, Source DB, and Target DB Schema Diagrams** with fluid viewport pan-and-zoom.
- **Copy SQL** migrations instantly using the clipboard copier.
- **Apply Migration** directly to sync your target schema!

*To change the UI port:*
```bash
./db-schema-differ --port 9090
```

---

### 3. Build from Source
Alternatively, you can compile the executable yourself using Go 1.20+:
```bash
make build
```
This compiles the executable to `bin/db-schema-differ`.

---

### 4. Configure Connections (Optional)
Specify DSN connection credentials via a `.env` file to auto-populate the UI forms:
```bash
cp .env.example .env
```
Open `.env` and specify `SOURCE_DSN` (desired dev schema DSN) and `TARGET_DSN` (current staging/prod schema DSN).

---

## Commands and Flags

```
db-schema-differ [command] [flags]

Commands:
  diff      Show schema differences between source and target databases
  generate  Generate SQL migration scripts based on differences
  apply     Apply schema differences directly to the target database
  version   Print the version number of db-schema-differ
```

### Global Flags
- `--config string`: Path to the config file (optional, defaults to `.env` or `differ.yaml`).
- `--driver string`: Database driver: `mysql` | `postgres` (default: `mysql`).
- `--source-dsn string`: Connection DSN for source database (desired state).
- `--target-dsn string`: Connection DSN for target database (current state).
- `--output-format string`: Output format: `terminal` | `json` | `html` (default: `terminal`).
- `--output-file string`: Save diff/migration output to a specific file.
- `--safe-only`: Skip any destructive actions (e.g. DROP column/table/index).
- `--no-color`: Disable colored terminal outputs.
- `--verbose`: Print detailed execution logs.

---

## Detailed Command Examples

### View HTML Diff Report
Generate a self-contained HTML page showing differences categorized by risk levels (SAFE, WARNING, DANGER):
```bash
./bin/db-schema-differ diff --output-format=html --output-file=report.html
```

### Generate Migration Script
Generate the migration SQL file:
```bash
./bin/db-schema-differ generate --output-file=migration.sql
```
To split safe additions and breaking drops into separate files:
```bash
./bin/db-schema-differ generate --split-breaking
```

### Dry-Run Apply
Preview DDL actions that would be executed on the target database without applying them:
```bash
./bin/db-schema-differ apply --dry-run
```

### Direct Apply (with Confirmation)
Compare, present summary, and apply differences to the target:
```bash
./bin/db-schema-differ apply
```

### CI/CD Auto-Confirm
Run without interactive prompts (ideal for automated deploy pipelines):
```bash
./bin/db-schema-differ apply --auto-confirm
```

---

## Dialect-Specific Features

### MySQL Non-Transactional DDL (State File Resume)
Because MySQL triggers an implicit commit for every DDL statement, if statement 5 of 10 fails, statements 1-4 remain applied.
1. When you run `apply`, `db-schema-differ` generates a `migration_state.json` file recording each pending DDL command.
2. In case of error, the applier displays the error and stops.
3. Fix the issue in the target database or source schema, and run:
   ```bash
   ./bin/db-schema-differ apply --resume
   ```
4. The tool will skip all successfully applied DDLs (marked `done` in the state file) and execute from the failed statement onwards.
5. On successful completion, the state file is automatically cleaned up.

### PostgreSQL Transaction Safety
For PostgreSQL, the applier wraps all statements in a single transaction block. If a statement fails:
1. The transaction is completely rolled back.
2. No partial changes are left in the target database.
3. Fix the issue and re-run the normal `apply` command.

---

## Testing

Run unit tests:
```bash
make test
```
To test connections, spin up the local Docker databases:
```bash
make up
# Set up schemas in source_db and target_db to test
make down
```
