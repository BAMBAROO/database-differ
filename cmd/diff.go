package cmd

import (
	"fmt"
	"os"

	"github.com/bryanathallah/db-schema-differ/config"
	"github.com/bryanathallah/db-schema-differ/internal/connector"
	"github.com/bryanathallah/db-schema-differ/internal/differ"
	"github.com/bryanathallah/db-schema-differ/internal/introspector"
	"github.com/bryanathallah/db-schema-differ/internal/reporter"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show schema differences between source and target databases",
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
		fmt.Fprintf(os.Stderr, "Connecting to Source Database: %s...\n", config.MaskDSN(cfg.SourceDSN, cfg.Driver))
		srcDB, err := conn.Connect(cfg.SourceDSN)
		if err != nil {
			return fmt.Errorf("source connection failure: %w", err)
		}
		defer srcDB.Close()

		if err := conn.ValidateReadPrivilege(srcDB); err != nil {
			return err
		}

		// Connect to Target
		fmt.Fprintf(os.Stderr, "Connecting to Target Database: %s...\n", config.MaskDSN(cfg.TargetDSN, cfg.Driver))
		tgtDB, err := conn.Connect(cfg.TargetDSN)
		if err != nil {
			return fmt.Errorf("target connection failure: %w", err)
		}
		defer tgtDB.Close()

		if err := conn.ValidateReadPrivilege(tgtDB); err != nil {
			return err
		}

		// 2. Introspect
		var intro introspector.Introspector
		if cfg.Driver == "mysql" {
			intro = introspector.NewMySQLIntrospector()
		} else {
			intro = introspector.NewPostgresIntrospector()
		}

		fmt.Fprintln(os.Stderr, "Introspecting Source schema...")
		srcSchema, err := intro.Introspect(srcDB, "")
		if err != nil {
			return fmt.Errorf("failed to introspect source DB: %w", err)
		}

		fmt.Fprintln(os.Stderr, "Introspecting Target schema...")
		tgtSchema, err := intro.Introspect(tgtDB, "")
		if err != nil {
			return fmt.Errorf("failed to introspect target DB: %w", err)
		}

		// 3. Diff
		fmt.Fprintln(os.Stderr, "Calculating differences...")
		schemaDiff, err := differ.Diff(srcSchema, tgtSchema)
		if err != nil {
			return fmt.Errorf("failed calculating diffs: %w", err)
		}

		// 4. Report
		var rep reporter.Reporter
		switch cfg.OutputFormat {
		case "terminal":
			rep = reporter.NewTerminalReporter(cfg.NoColor)
		case "json":
			rep = reporter.NewJSONReporter()
		case "html":
			rep = reporter.NewHTMLReporter()
		default:
			rep = reporter.NewTerminalReporter(cfg.NoColor)
		}

		outWriter := os.Stdout
		if cfg.OutputFile != "" {
			f, err := os.Create(cfg.OutputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file %s: %w", cfg.OutputFile, err)
			}
			defer f.Close()
			outWriter = f
			fmt.Fprintf(os.Stderr, "Writing diff report to %s...\n", cfg.OutputFile)
		}

		return rep.Report(schemaDiff, outWriter)
	},
}

func init() {
	RootCmd.AddCommand(diffCmd)
}
