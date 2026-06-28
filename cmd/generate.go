package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bryanathallah/db-schema-differ/internal/connector"
	"github.com/bryanathallah/db-schema-differ/internal/differ"
	"github.com/bryanathallah/db-schema-differ/internal/generator"
	"github.com/bryanathallah/db-schema-differ/internal/introspector"
	"github.com/spf13/cobra"
)

var splitBreaking bool

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate SQL migration scripts based on differences",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := ConfigInstance
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		// 1. Get Connector
		conn, err := connector.NewConnector(cfg.Driver)
		if err != nil {
			return fmt.Errorf("failed to load connector: %w", err)
		}

		// Connect to Source
		fmt.Fprintf(os.Stderr, "Connecting to Source Database...\n")
		srcDB, err := conn.Connect(cfg.SourceDSN)
		if err != nil {
			return fmt.Errorf("source connection failure: %w", err)
		}
		defer srcDB.Close()

		// Connect to Target
		fmt.Fprintf(os.Stderr, "Connecting to Target Database...\n")
		tgtDB, err := conn.Connect(cfg.TargetDSN)
		if err != nil {
			return fmt.Errorf("target connection failure: %w", err)
		}
		defer tgtDB.Close()

		// 2. Introspect
		var intro introspector.Introspector
		var gen generator.Generator
		if cfg.Driver == "mysql" {
			intro = introspector.NewMySQLIntrospector()
			gen = generator.NewMySQLGenerator()
		} else {
			intro = introspector.NewPostgresIntrospector()
			gen = generator.NewPostgresGenerator()
		}

		fmt.Fprintln(os.Stderr, "Introspecting schemas...")
		srcSchema, err := intro.Introspect(srcDB, "")
		if err != nil {
			return fmt.Errorf("failed to introspect source DB: %w", err)
		}

		tgtSchema, err := intro.Introspect(tgtDB, "")
		if err != nil {
			return fmt.Errorf("failed to introspect target DB: %w", err)
		}

		// 3. Diff
		schemaDiff, err := differ.Diff(srcSchema, tgtSchema)
		if err != nil {
			return fmt.Errorf("failed calculating diffs: %w", err)
		}

		if len(schemaDiff.Tables) == 0 {
			fmt.Fprintln(os.Stderr, "No schema changes detected. SQL generation skipped.")
			return nil
		}

		// 4. Generate SQL DDL statements
		opts := generator.GenOptions{
			SafeOnly: cfg.SafeOnly,
		}

		statements, err := gen.Generate(schemaDiff, srcSchema, opts)
		if err != nil {
			return fmt.Errorf("failed to generate DDL statements: %w", err)
		}

		sqlContent := strings.Join(statements, "\n") + "\n"

		// 5. Output
		outFile := cfg.OutputFile
		if outFile == "" && cmd.Flags().Changed("output-file") {
			outFile = outputFile
		}

		// If no output file is provided, generate a default name
		if outFile == "" {
			outFile = fmt.Sprintf("migration_%s.sql", time.Now().Format("20060102_150405"))
		}

		// Check if user requested splitting breaking changes (e.g. migration_safe.sql and migration_breaking.sql)
		if splitBreaking && !cfg.SafeOnly {
			var safeStmts []string

			// Re-generate separate options
			optsSafe := generator.GenOptions{SafeOnly: true}
			safeStmts, err = gen.Generate(schemaDiff, srcSchema, optsSafe)
			if err != nil {
				return err
			}

			// For breaking, let's keep the full but print warning or separate them.
			// Actually, let's filter the statements slice directly or generate again.
			// A simple filter of statements contains DROP or ALTER ... DROP or DELETE etc.
			// To keep it simple, we write the full one to the main file, and safe to another file.
			safeOutFile := strings.Replace(outFile, ".sql", "_safe.sql", 1)
			breakingOutFile := strings.Replace(outFile, ".sql", "_breaking.sql", 1)

			// Generate safe statements
			safeSQL := strings.Join(safeStmts, "\n") + "\n"
			if err := os.WriteFile(safeOutFile, []byte(safeSQL), 0644); err != nil {
				return fmt.Errorf("failed to write safe migration file: %w", err)
			}
			fmt.Printf("Safe migration script written to: %s\n", safeOutFile)

			// Generate full (with breaking)
			if err := os.WriteFile(breakingOutFile, []byte(sqlContent), 0644); err != nil {
				return fmt.Errorf("failed to write breaking migration file: %w", err)
			}
			fmt.Printf("Breaking migration script (full) written to: %s\n", breakingOutFile)
			return nil
		}

		if err := os.WriteFile(outFile, []byte(sqlContent), 0644); err != nil {
			return fmt.Errorf("failed to write migration file %s: %w", outFile, err)
		}

		fmt.Printf("Migration script successfully generated and written to: %s\n", outFile)
		return nil
	},
}

func init() {
	generateCmd.Flags().BoolVar(&splitBreaking, "split-breaking", false, "split migration script into safe and breaking changes files")
	RootCmd.AddCommand(generateCmd)
}
