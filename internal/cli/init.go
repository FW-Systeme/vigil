package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FW-Systeme/Virgil/internal/process"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate an ecosystem.json template",
		RunE: func(cmd *cobra.Command, args []string) error {
		tmpl := process.Process{
			Name:    "my-app",
			Type:    process.TypeApp,
			Command: "./my-app",
			Port:    3000,
		}

			data, err := json.MarshalIndent(tmpl, "", "  ")
			if err != nil {
				return fmt.Errorf("generating template: %w", err)
			}

			if err := os.WriteFile(output, data, 0600); err != nil {
				return fmt.Errorf("writing template: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote template to %s\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "ecosystem.json", "Output file path")

	return cmd
}
