package cli

import (
	"fmt"
	"io"

	"github.com/chris576/vigil/internal/update"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var version string
	var quiet bool

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an app to a new release",
		Long: `Update a registered app from a .tar.gz package.

The update process:
  1. Locks the app's working directory
  2. Ensures releases/, shared/, incoming/ exist
  3. Resolves the version (from flag or incoming/ scan)
  4. Verifies package integrity (SHA256)
  5. Extracts the archive
  6. Installs dependencies (unless --bundled-deps)
  7. Links shared/ data into the release
  8. Switches the current symlink atomically
  9. Restarts the service
  10. Runs the smoke test script (rollback on failure)
  11. Cleans up old releases (keeps last 3)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}

			var out io.Writer = cmd.OutOrStdout()
			if quiet {
				out = io.Discard
			}

			svc := update.NewService(pm.Store(), pm.RestartProcess, out)

			if err := svc.Update(cmd.Context(), args[0], version); err != nil {
				return err
			}

			if !quiet {
				if version != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %q to %s\n", args[0], version)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %q\n", args[0])
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Target version (auto-detect from incoming/ if empty)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress progress output")

	return cmd
}
