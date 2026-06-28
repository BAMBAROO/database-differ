# MySQL Limitations and Workarounds (Implicit Commit Behavior)

In MySQL, executing DDL statements (`CREATE TABLE`, `ALTER TABLE`, `DROP TABLE`, etc.) triggers an **implicit commit**. This means that unlike PostgreSQL, **MySQL does not support transactional DDL**.

If a migration contains 10 DDL statements and statement 5 fails (e.g., due to a duplicate column name or invalid syntax):
- Statements 1 to 4 have already been applied and committed to the database.
- They **cannot** be rolled back automatically.
- Re-running the entire migration script will fail at statement 1 because the table or column already exists.

---

## The Solution: State File & `--resume`

To handle this behavior, `db-schema-differ` uses a state-file tracking mechanism for MySQL:

1. **State Initialization**: When you run `apply`, the tool first parses the statements and generates a `migration_state.json` file. All statements are initialized as `pending`.
2. **Step-by-step Execution**: The tool executes statements one by one. After each successful statement, its status in the state file is updated to `done`.
3. **Failure Stop**: If a statement fails, its status is updated to `failed`, execution stops immediately, and a clear error is shown explaining where it failed and why.
4. **Resolution**: Fix the issue either in your target database (e.g. by dropping an conflicting column) or in the source schema.
5. **Resume**: Run the apply command with the `--resume` flag:
   ```bash
   ./bin/db-schema-differ apply --resume
   ```
6. **State Resumption**: The tool reads the `migration_state.json` file, skips all statements already marked `done`, and continues execution starting from the failed statement.
7. **Clean up**: Once all statements are successfully applied, the `migration_state.json` file is deleted automatically.

---

## Example State File (`migration_state.json`)

Here is what the state file looks like during a migration:

```json
{
  "generated_at": "2026-06-28T14:30:00Z",
  "driver": "mysql",
  "statements": [
    {
      "id": 1,
      "sql": "CREATE TABLE `users` ( `id` int NOT NULL, PRIMARY KEY (`id`) );",
      "status": "done"
    },
    {
      "id": 2,
      "sql": "ALTER TABLE `users` ADD COLUMN `email` varchar(255) NULL;",
      "status": "failed"
    },
    {
      "id": 3,
      "sql": "CREATE TABLE `orders` ( `id` int NOT NULL, PRIMARY KEY (`id`) );",
      "status": "pending"
    }
  ]
}
```
In this scenario, running `./bin/db-schema-differ apply --resume` will skip statement 1 and execute statements 2 and 3.
