package cmd

import (
	"fmt"
	"os"

	"github.com/bryanathallah/db-schema-differ/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile      string
	sourceDSN    string
	targetDSN    string
	driver       string
	noColor      bool
	verbose      bool
	safeOnly     bool
	outputFormat string
	outputFile   string

	// ConfigInstance is the globally parsed configuration.
	ConfigInstance *config.Config
)

var RootCmd = &cobra.Command{
	Use:   "db-schema-differ",
	Short: "DB Schema Differ & Migrator compares and syncs database schemas",
	Long: `A powerful CLI tool to introspect two database schemas (MySQL or PostgreSQL),
calculate schema differences, generate migrations, and safely apply them.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config from path (if any) or defaults
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return err
		}

		// Bind command line overrides to the config struct if they were set
		if cmd.Flags().Changed("source-dsn") {
			cfg.SourceDSN = sourceDSN
		}
		if cmd.Flags().Changed("target-dsn") {
			cfg.TargetDSN = targetDSN
		}
		if cmd.Flags().Changed("driver") {
			cfg.Driver = driver
		}
		if cmd.Flags().Changed("no-color") {
			cfg.NoColor = noColor
		}
		if cmd.Flags().Changed("verbose") {
			cfg.Verbose = verbose
		}
		if cmd.Flags().Changed("safe-only") {
			cfg.SafeOnly = safeOnly
		}
		if cmd.Flags().Changed("output-format") {
			cfg.OutputFormat = outputFormat
		}
		if cmd.Flags().Changed("output-file") {
			cfg.OutputFile = outputFile
		}

		// Re-run validation on the resolved configurations
		if err := cfg.Validate(); err != nil {
			// Do not fail root validation if subcommands like 'version' are called,
			// let individual commands check as needed.
			if cmd.Name() != "version" && cmd.Name() != "help" {
				return err
			}
		}

		ConfigInstance = cfg
		return nil
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: .env or differ.yaml)")
	RootCmd.PersistentFlags().StringVar(&sourceDSN, "source-dsn", "", "source database connection string (desired state)")
	RootCmd.PersistentFlags().StringVar(&targetDSN, "target-dsn", "", "target database connection string (current state)")
	RootCmd.PersistentFlags().StringVar(&driver, "driver", "mysql", "database driver: mysql | postgres")
	RootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored terminal output")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable detailed log outputs")
	RootCmd.PersistentFlags().BoolVar(&safeOnly, "safe-only", false, "only execute or generate safe schema penambahan (no drops)")
	RootCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "terminal", "output format: terminal | sql | html | json")
	RootCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "output file path (optional)")
}

func initConfig() {
	// Viper binding for automatic environment variables
	viper.AutomaticEnv()
}
