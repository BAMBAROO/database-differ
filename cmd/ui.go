package cmd

import (
	"fmt"
	"os/exec"
	"os"
	"runtime"
	"time"

	"github.com/bryanathallah/db-schema-differ/internal/ui"
	"github.com/spf13/cobra"
)

var port int

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start the interactive Web UI dashboard",
	Long:  `Launches a local HTTP server hosting a premium, self-contained dashboard.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := ConfigInstance
		if cfg == nil {
			return fmt.Errorf("configuration not loaded")
		}

		server := ui.NewServer(cfg)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		url := fmt.Sprintf("http://%s", addr)

		fmt.Printf("Starting Web UI Server on %s...\n", url)

		// Start a goroutine to open browser after server starts
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(url)
		}()

		if err := server.ListenAndServe(addr); err != nil {
			return fmt.Errorf("web server failed: %w", err)
		}

		return nil
	},
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically. Please open in your browser: %s\n", url)
	}
}

func init() {
	uiCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run the UI server on")
	RootCmd.AddCommand(uiCmd)
}
