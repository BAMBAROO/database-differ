# Configuration Guide

`db-schema-differ` can be configured using three methods, ordered by priority (highest to lowest):
1. **Command Line Flags** (e.g. `--source-dsn="..."`)
2. **Environment Variables** (loaded from `.env` file or local shell environment)
3. **YAML Config File** (loaded from `differ.yaml` if present)

---

## Configuration Variables

| Config Name | CLI Flag | Type | Default | Description |
|---|---|---|---|---|
| `DB_DRIVER` | `--driver` | string | `mysql` | Target database driver: `mysql` or `postgres`. |
| `SOURCE_DSN` | `--source-dsn` | string | | Source database connection string (desired state). |
| `TARGET_DSN` | `--target-dsn` | string | | Target database connection string (current state). |
| `OUTPUT_FORMAT`| `--output-format`| string | `terminal`| Output format: `terminal`, `sql`, `html`, `json`. |
| `OUTPUT_FILE` | `--output-file` | string | | Save report/script to file instead of printing. |
| `AUTO_APPLY` | `--auto-confirm`| bool | `false` | Skip interactive prompts before applying changes. |
| `DRY_RUN` | `--dry-run` | bool | `false` | Preview generated SQL without modifying target DB. |
| `SAFE_ONLY` | `--safe-only` | bool | `false` | Disable any destructive actions (skips DROP statements).|
| `NO_COLOR` | `--no-color` | bool | `false` | Disable ANSI color codes in terminal outputs. |
| `VERBOSE` | `--verbose` | bool | `false` | Print detailed internal logs. |

---

## Example Configurations

### 1. `.env` File
Create a `.env` file in the directory where you run the tool:
```ini
DB_DRIVER=mysql
SOURCE_DSN="root:rootpassword@tcp(127.0.0.1:13306)/source_db?parseTime=true"
TARGET_DSN="root:rootpassword@tcp(127.0.0.1:13306)/target_db?parseTime=true"
OUTPUT_FORMAT=terminal
SAFE_ONLY=false
```

### 2. `differ.yaml` File
Alternatively, create a `differ.yaml` file in the current directory:
```yaml
db_driver: postgres
source_dsn: "postgres://postgres:postgrespassword@127.0.0.1:15432/source_db?sslmode=disable"
target_dsn: "postgres://postgres:postgrespassword@127.0.0.1:15432/target_db?sslmode=disable"
output_format: html
output_file: report.html
```

### 3. Command Line Overrides
CLI flags override variables specified in `.env` or `differ.yaml`:
```bash
# Force postgres driver and terminal output format
./bin/db-schema-differ diff --driver=postgres --output-format=terminal
```
