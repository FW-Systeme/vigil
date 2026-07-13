package cli

import (
	"fmt"

	"github.com/chris576/vigil/internal/nginx"
	"github.com/chris576/vigil/internal/process"
	"github.com/chris576/vigil/internal/systemd"
	"github.com/spf13/cobra"
)

var version = "dev"

func SetVersion(v string) {
	version = v
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
			if c, err := systemd.New(); err == nil {
				sdClient = c
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
