package cli

import (
	"fmt"
	"strings"

	"github.com/FW-Systeme/Virgil/internal/nginx"
	"github.com/FW-Systeme/Virgil/internal/process"
	"github.com/FW-Systeme/Virgil/internal/systemd"
	"github.com/spf13/cobra"
)

var version = "dev"

func SetVersion(v string) {
	version = v
}

// needsSystemd reports whether cmd requires a systemd D-Bus connection.
// Commands that only touch the store, crontab, or local files return false.
func needsSystemd(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	switch {
	case path == "vigil":
		return false
	case path == "vigil list":
		return false
	case path == "vigil init":
		return false
	case path == "vigil version":
		return false
	case strings.HasPrefix(path, "vigil cron"):
		return false
	case path == "vigil logsave status":
		return false
	default:
		return true
	}
}

func isHelpOrVersion(cmd *cobra.Command) bool {
	path := cmd.CommandPath()
	return path == "vigil" || path == "vigil version"
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vigil",
		Short: "A lightweight process manager (PM2 alternative)",
		Long:  `Vigil is a lightweight CLI process manager that wraps systemd and nginx.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			store, err := process.NewStore()
			if err != nil {
				return fmt.Errorf("initializing store: %w", err)
			}

			var sdClient systemd.Client
			var sdErr error
			if c, err := systemd.New(); err == nil {
				sdClient = c
			} else {
				sdErr = err
			}

			if sdClient == nil && needsSystemd(cmd) {
				return fmt.Errorf("systemd unavailable (try running with sudo): %w", sdErr)
			} else if sdErr != nil && !isHelpOrVersion(cmd) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: systemd unavailable: %v\n", sdErr)
			}

			var nginxClient nginx.Client
			if c, err := nginx.New(); err == nil {
				nginxClient = c
			}

			pm := process.New(store, sdClient, nginxClient)
			cmd.SetContext(pmCtx(cmd.Context(), pm))
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newLogSaveCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newCronCmd())

	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version)
		},
	})

	return cmd
}

func Execute() error {
	return NewRootCmd().Execute()
}
