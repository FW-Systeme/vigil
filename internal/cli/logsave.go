package cli

import (
	"fmt"
	"path/filepath"

	"github.com/FW-Systeme/Virgil/internal/process"
	"github.com/spf13/cobra"
)

func newLogSaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logsave",
		Short: "Manage persistent log saving",
	}
	cmd.AddCommand(newLogSaveEnableCmd())
	cmd.AddCommand(newLogSaveDisableCmd())
	cmd.AddCommand(newLogSaveStatusCmd())
	return cmd
}

func newLogSaveEnableCmd() *cobra.Command {
	var maxSize string
	var outputDir string
	var rotate int

	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable persistent log saving",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}
			name := args[0]
			logPath := filepath.Join(outputDir, name+".log")
			if err := pm.SetupLogging(cmd.Context(), name, logPath, maxSize, rotate); err != nil {
				return err
			}
			ls, err := process.NewLogStore()
			if err != nil {
				return err
			}
			if err := ls.Save(process.LogConfig{
				Name:    name,
				Enabled: true,
				LogPath: logPath,
				MaxSize: maxSize,
				Rotate:  rotate,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled log saving for %q (%s)\n", name, logPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&maxSize, "max-size", "10M", "Max log file size before rotation (e.g. 10M, 1G)")
	cmd.Flags().StringVar(&outputDir, "output", "/var/log/vigil", "Log output directory")
	cmd.Flags().IntVar(&rotate, "rotate", 3, "Number of rotated files to keep")
	return cmd
}

func newLogSaveDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable persistent log saving",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}
			name := args[0]
			if err := pm.RemoveLogging(cmd.Context(), name); err != nil {
				return err
			}
			ls, err := process.NewLogStore()
			if err != nil {
				return err
			}
			if err := ls.Delete(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Disabled log saving for %q\n", name)
			return nil
		},
	}
}

func newLogSaveStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show log save status for an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ls, err := process.NewLogStore()
			if err != nil {
				return err
			}
			name := args[0]
			cfg, err := ls.Load(name)
			if err != nil {
				return fmt.Errorf("loading log config: %w", err)
			}
			if !cfg.Enabled {
				fmt.Fprintf(cmd.OutOrStdout(), "Log saving for %q: disabled\n", name)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "App:       %s\n", cfg.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled:   %v\n", cfg.Enabled)
			fmt.Fprintf(cmd.OutOrStdout(), "Log Path:  %s\n", cfg.LogPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Max Size:  %s\n", cfg.MaxSize)
			fmt.Fprintf(cmd.OutOrStdout(), "Rotate:    %d\n", cfg.Rotate)
			return nil
		},
	}
}
