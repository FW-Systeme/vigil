package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			// Set up log output before any other output.
			if logOutputPath, _ := cmd.Flags().GetString("log-output"); logOutputPath != "" {
				f, err := openLogFile(logOutputPath, cmd.CommandPath())
				if err != nil {
					return fmt.Errorf("opening log file: %w", err)
				}
				cmd.SetContext(contextWithLogFile(cmd.Context(), f))

				outW := cmd.OutOrStdout()
				cmd.SetOut(io.MultiWriter(outW, f))
				errW := cmd.ErrOrStderr()
				cmd.SetErr(io.MultiWriter(errW, f))

				if root := cmd.Root(); root != cmd {
					root.SetOut(io.MultiWriter(root.OutOrStdout(), f))
					root.SetErr(io.MultiWriter(root.ErrOrStderr(), f))
				}
			}

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
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if f := logFileFromCtx(cmd.Context()); f != nil {
				f.Close()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().String("log-output", "", "Log command stdout/stderr to a directory or file")

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newRestartCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLogsCmd())
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

func openLogFile(path, cmdPath string) (*os.File, error) {
	fi, err := os.Stat(path)
	if err == nil && fi.IsDir() {
		return openLogInDir(path, cmdPath)
	}
	if err == nil {
		return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, err
		}
		return openLogInDir(path, cmdPath)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func openLogInDir(dir, cmdPath string) (*os.File, error) {
	name := filepath.Join(dir,
		fmt.Sprintf("%s-%s-%d.log",
			strings.ReplaceAll(cmdPath, " ", "-"),
			time.Now().Format("20060102-150405"),
			os.Getpid(),
		),
	)
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func contextWithLogFile(ctx context.Context, f *os.File) context.Context {
	return context.WithValue(ctx, logFileKey, f)
}

func logFileFromCtx(ctx context.Context) *os.File {
	f, _ := ctx.Value(logFileKey).(*os.File)
	return f
}
