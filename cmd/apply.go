package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bryanathallah/db-schema-differ/config"
	"github.com/bryanathallah/db-schema-differ/internal/applier"
	"github.com/bryanathallah/db-schema-differ/internal/connector"
	"github.com/bryanathallah/db-schema-differ/internal/differ"
	"github.com/bryanathallah/db-schema-differ/internal/generator"
	"github.com/bryanathallah/db-schema-differ/internal/introspector"
	"github.com/bryanathallah/db-schema-differ/models"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	dryRun        bool
	autoConfirm   bool
	resume        bool
	stateFilePath string
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply schema differences directly to the target database",
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

		// Connect to Target DB
		fmt.Fprintf(os.Stderr, "Connecting to Target Database: %s...\n", config.MaskDSN(cfg.TargetDSN, cfg.Driver))
		tgtDB, err := conn.Connect(cfg.TargetDSN)
		if err != nil {
			return fmt.Errorf("target connection failure: %w", err)
		}
		defer tgtDB.Close()

		var statements []string
		var srcSchema *models.Schema

		// If --resume is specified (MySQL only), we don't need to introspect/generate anew,
		// we just load the statements directly from the state file.
		if resume {
			if cfg.Driver != "mysql" {
				return fmt.Errorf("the --resume flag is only supported on mysql databases")
			}
			// We pass empty statements slice since MySQLApplier will read them from the state file
			statements = []string{}
		} else {
			// Connect to Source DB
			fmt.Fprintf(os.Stderr, "Connecting to Source Database: %s...\n", config.MaskDSN(cfg.SourceDSN, cfg.Driver))
			srcDB, err := conn.Connect(cfg.SourceDSN)
			if err != nil {
				return fmt.Errorf("source connection failure: %w", err)
			}
			defer srcDB.Close()

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
			srcSchema, err = intro.Introspect(srcDB, "")
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
				fmt.Println("No differences found. Database schemas are already in sync!")
				return nil
			}

			// 4. Generate DDL
			opts := generator.GenOptions{
				SafeOnly: cfg.SafeOnly,
			}
			statements, err = gen.Generate(schemaDiff, srcSchema, opts)
			if err != nil {
				return fmt.Errorf("failed generating DDL: %w", err)
			}

			// 5. Interactive Confirmation
			if !autoConfirm && !dryRun {
				// Show Risk warnings
				hasDanger := schemaDiff.Summary.DangersCount > 0
				hasWarnings := schemaDiff.Summary.WarningsCount > 0

				if hasDanger {
					color.Red("⚠️  WARNING: %d DANGER statements detected (DROP operations). Data loss will occur!", schemaDiff.Summary.DangersCount)
				} else if hasWarnings {
					color.Yellow("⚠️  WARNING: %d warning statements detected (possible type changes/nullability changes).", schemaDiff.Summary.WarningsCount)
				}

				msg := fmt.Sprintf("Continue and apply changes to target database? [y/N]: ")
				if !askConfirm(msg) {
					fmt.Println("Apply cancelled by user.")
					return nil
				}

				// If there are DANGER changes, ask a second time
				if hasDanger {
					color.Red("❗ CRITICAL WARNING: You are performing destructive operations on the target database.")
					if !askConfirmText("Please type 'yes' to proceed with breaking changes: ", "yes") {
						fmt.Println("Apply cancelled by user.")
						return nil
					}
				}
			}
		}

		// 6. Dry Run Mode
		if dryRun {
			fmt.Println("\n--- DRY RUN: SQL statements to be applied ---")
			for _, stmt := range statements {
				fmt.Println(stmt)
			}
			fmt.Println("-------------------------------------------")
			fmt.Println("Dry run completed. No changes were made to the database.")
			return nil
		}

		// 7. Get Applier
		var apply applier.Applier
		if cfg.Driver == "mysql" {
			apply = applier.NewMySQLApplier()
			// Log a warnings notice for MySQL implicit DDL commits
			color.Yellow("\n⚠️  MySQL driver selected: DDL actions are non-transactional and cannot be rolled back.")
			fmt.Println("   A state file will be generated to support resuming if a failure occurs.")
		} else {
			apply = applier.NewPostgresApplier()
		}

		// 8. Execute Apply
		err = apply.Apply(tgtDB, statements, stateFilePath, resume)
		if err != nil {
			return err
		}

		// 9. Post-apply verification: introspect source and target again to verify they match!
		if !resume {
			fmt.Println("\nVerifying database schemas are in sync...")
			var intro introspector.Introspector
			if cfg.Driver == "mysql" {
				intro = introspector.NewMySQLIntrospector()
			} else {
				intro = introspector.NewPostgresIntrospector()
			}

			// We need to re-open DB or just query
			newTgtSchema, err := intro.Introspect(tgtDB, "")
			if err == nil {
				newDiff, err := differ.Diff(srcSchema, newTgtSchema)
				if err == nil {
					if len(newDiff.Tables) == 0 {
						color.Green("✓ Verification Success: Database schemas are now identical.")
					} else {
						color.Yellow("⚠ Verification Warning: Schemas still differ slightly after applying changes.")
					}
				}
			}
		}

		return nil
	},
}

func askConfirm(message string) bool {
	fmt.Print(message)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func askConfirmText(message string, expected string) bool {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == expected
}

func init() {
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview SQL actions without executing them")
	applyCmd.Flags().BoolVar(&autoConfirm, "auto-confirm", false, "skip interactive confirmation prompts (useful for CI/CD)")
	applyCmd.Flags().BoolVar(&resume, "resume", false, "resume execution from state file after a partial failure (MySQL only)")
	applyCmd.Flags().StringVar(&stateFilePath, "state-file", "migration_state.json", "custom state file path for MySQL resume logic")

	RootCmd.AddCommand(applyCmd)
}
