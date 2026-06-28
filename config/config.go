package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Driver       string `mapstructure:"DB_DRIVER"`
	SourceDSN    string `mapstructure:"SOURCE_DSN"`
	TargetDSN    string `mapstructure:"TARGET_DSN"`
	OutputFormat string `mapstructure:"OUTPUT_FORMAT"`
	OutputFile   string `mapstructure:"OUTPUT_FILE"`
	AutoApply    bool   `mapstructure:"AUTO_APPLY"`
	DryRun       bool   `mapstructure:"DRY_RUN"`
	Verbose      bool   `mapstructure:"VERBOSE"`
	NoColor      bool   `mapstructure:"NO_COLOR"`
	SafeOnly     bool   `mapstructure:"SAFE_ONLY"`
}

// LoadConfig reads configuration from file (if provided/exists), environment variables, and parses it.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Default values
	v.SetDefault("DB_DRIVER", "mysql")
	v.SetDefault("OUTPUT_FORMAT", "terminal")
	v.SetDefault("AUTO_APPLY", false)
	v.SetDefault("DRY_RUN", false)
	v.SetDefault("VERBOSE", false)
	v.SetDefault("NO_COLOR", false)
	v.SetDefault("SAFE_ONLY", false)

	// Enable environment variables reading
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Load from .env file if it exists in the current directory
	v.AddConfigPath(".")
	v.SetConfigName(".env")
	v.SetConfigType("env")
	if err := v.ReadInConfig(); err != nil {
		// It's okay if .env doesn't exist, we fallback to env vars / defaults
	}

	// Load specific config file if path is specified
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.MergeInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	} else {
		// Alternatively, look for differ.yaml in current directory
		v.SetConfigName("differ")
		v.SetConfigType("yaml")
		_ = v.MergeInConfig() // ignore error if differ.yaml is not present
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configurations are complete and valid.
func (c *Config) Validate() error {
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if c.Driver != "mysql" && c.Driver != "postgres" {
		return fmt.Errorf("invalid DB_DRIVER: '%s'. Supported drivers are 'mysql' or 'postgres'", c.Driver)
	}

	if strings.TrimSpace(c.SourceDSN) == "" {
		return fmt.Errorf("SOURCE_DSN cannot be empty")
	}

	if strings.TrimSpace(c.TargetDSN) == "" {
		return fmt.Errorf("TARGET_DSN cannot be empty")
	}

	c.OutputFormat = strings.ToLower(strings.TrimSpace(c.OutputFormat))
	validFormats := map[string]bool{"terminal": true, "sql": true, "html": true, "json": true}
	if !validFormats[c.OutputFormat] {
		return fmt.Errorf("invalid OUTPUT_FORMAT: '%s'. Supported formats are: terminal, sql, html, json", c.OutputFormat)
	}

	return nil
}

// MaskDSN returns a copy of the DSN with password obfuscated.
func MaskDSN(dsn string, driver string) string {
	if driver == "mysql" {
		// Format: username:password@protocol(address)/dbname?param=value
		parts := strings.SplitN(dsn, "@", 2)
		if len(parts) < 2 {
			return dsn // Can't parse, return as is
		}
		userPass := strings.SplitN(parts[0], ":", 2)
		if len(userPass) < 2 {
			return dsn
		}
		return fmt.Sprintf("%s:***@%s", userPass[0], parts[1])
	} else if driver == "postgres" {
		// Format: postgres://username:password@host:port/dbname?query
		if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
			return dsn
		}
		prefix := "postgres://"
		if strings.HasPrefix(dsn, "postgresql://") {
			prefix = "postgresql://"
		}
		rest := dsn[len(prefix):]
		parts := strings.SplitN(rest, "@", 2)
		if len(parts) < 2 {
			return dsn
		}
		userPass := strings.SplitN(parts[0], ":", 2)
		if len(userPass) < 2 {
			return dsn
		}
		return fmt.Sprintf("%s%s:***@%s", prefix, userPass[0], parts[1])
	}
	return dsn
}
