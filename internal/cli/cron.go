package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chris576/vigil/internal/cron"
	"github.com/spf13/cobra"
)

func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage scheduled cron jobs",
	}
	cmd.AddCommand(newCronInitCmd())
	cmd.AddCommand(newCronAddCmd())
	cmd.AddCommand(newCronListCmd())
	cmd.AddCommand(newCronRemoveCmd())
	cmd.AddCommand(newCronEnableCmd())
	cmd.AddCommand(newCronDisableCmd())
	cmd.AddCommand(newCronStatusCmd())
	return cmd
}

func initCronCtx(cmd *cobra.Command) (cron.Store, cron.Client, error) {
	store, client, ok := cronFromCtx(cmd.Context())
	if ok {
		return store, client, nil
	}
	store, err := cron.NewStore()
	if err != nil {
		return nil, nil, fmt.Errorf("initializing cron store: %w", err)
	}
	return store, cron.NewClient(), nil
}

func newCronInitCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a cronfile.json template",
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpl := cron.CronFile{
				Crons: []cron.Job{
					{
						Name:     "my-job",
						Schedule: "0 3 * * *",
						Command:  "/path/to/script.sh",
						Enabled:  true,
					},
				},
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

	cmd.Flags().StringVarP(&output, "output", "o", "cronfile.json", "Output file path")
	return cmd
}

func newCronAddCmd() *cobra.Command {
	var schedule string
	var command string
	var configFile string

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a cron job",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if configFile != "" {
				return addCronFromConfig(cmd, configFile, args)
			}

			if len(args) < 1 {
				return fmt.Errorf("accepts 1 arg(s), received 0")
			}

			name := args[0]
			if schedule == "" {
				return fmt.Errorf("required flag(s) \"schedule\" not set")
			}
			if command == "" {
				return fmt.Errorf("required flag(s) \"command\" not set")
			}

			store, client, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			if _, err := store.Get(name); err == nil {
				return fmt.Errorf("cron job %q already exists in store", name)
			}

			job := cron.Job{
				Name:      name,
				Schedule:  schedule,
				Command:   command,
				Enabled:   true,
				CreatedAt: time.Now(),
			}

			if err := job.Validate(); err != nil {
				return err
			}

			if err := store.Save(job); err != nil {
				return fmt.Errorf("saving cron job: %w", err)
			}

			if err := client.Install(job); err != nil {
				return fmt.Errorf("installing crontab entry: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added cron job %q (%s)\n", name, schedule)
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&schedule, "schedule", "", "Cron schedule expression (e.g. \"0 3 * * *\")")
	flags.StringVar(&command, "command", "", "Command or script to execute")
	flags.StringVar(&configFile, "config", "", "Path to cronfile JSON")
	return cmd
}

func addCronFromConfig(cmd *cobra.Command, path string, args []string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening cron file: %w", err)
	}
	defer f.Close()

	jobs, err := cron.ParseCronFile(f)
	if err != nil {
		return err
	}

	var filterName string
	if len(args) > 0 {
		filterName = args[0]
	}

	store, client, err := initCronCtx(cmd)
	if err != nil {
		return err
	}

	var added, errors int
	var matched bool
	for _, job := range jobs {
		if filterName != "" && job.Name == filterName {
			matched = true
		}
		if filterName != "" && job.Name != filterName {
			continue
		}

		if err := job.Validate(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skipping %q: %v\n", job.Name, err)
			errors++
			continue
		}

		if _, err := store.Get(job.Name); err == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skipping %q: already exists in store\n", job.Name)
			errors++
			continue
		}

		if job.CreatedAt.IsZero() {
			job.CreatedAt = time.Now()
		}

		if err := store.Save(job); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %q: %v\n", job.Name, err)
			errors++
			continue
		}

		if err := client.Install(job); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error installing crontab %q: %v\n", job.Name, err)
			errors++
			continue
		}

		added++
	}

	if filterName != "" && !matched {
		fmt.Fprintf(cmd.OutOrStdout(), "0 cron job(s) added, 0 error(s)\n")
		return fmt.Errorf("cron job %q not found in config file", filterName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%d cron job(s) added, %d error(s)\n", added, errors)

	if errors > 0 {
		return fmt.Errorf("%d error(s) occurred", errors)
	}
	return nil
}

func newCronListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured cron jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			jobs, err := store.List()
			if err != nil {
				return fmt.Errorf("listing cron jobs: %w", err)
			}

			if len(jobs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cron jobs configured")
				return nil
			}

			for _, job := range jobs {
				status := "enabled"
				if !job.Enabled {
					status = "disabled"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-20s %s\n", job.Name, job.Schedule, status, job.Command)
			}
			return nil
		},
	}
}

func newCronRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store, client, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			if err := store.Delete(name); err != nil {
				return fmt.Errorf("deleting cron job from store: %w", err)
			}

			if err := client.Remove(name); err != nil && err != cron.ErrNotFound {
				return fmt.Errorf("removing crontab entry: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed cron job %q\n", name)
			return nil
		},
	}
}

func newCronEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store, client, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			job, err := store.Get(name)
			if err != nil {
				return fmt.Errorf("cron job %q not found in store", name)
			}

			if err := client.Enable(name); err != nil {
				return fmt.Errorf("enabling crontab entry: %w", err)
			}

			job.Enabled = true
			if err := store.Save(job); err != nil {
				return fmt.Errorf("updating cron job: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Enabled cron job %q\n", name)
			return nil
		},
	}
}

func newCronDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a cron job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store, client, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			job, err := store.Get(name)
			if err != nil {
				return fmt.Errorf("cron job %q not found in store", name)
			}

			if err := client.Disable(name); err != nil {
				return fmt.Errorf("disabling crontab entry: %w", err)
			}

			job.Enabled = false
			if err := store.Save(job); err != nil {
				return fmt.Errorf("updating cron job: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Disabled cron job %q\n", name)
			return nil
		},
	}
}

func newCronStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show cron job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store, client, err := initCronCtx(cmd)
			if err != nil {
				return err
			}

			running, err := client.IsCronRunning()
			if err != nil || !running {
				fmt.Fprintf(cmd.OutOrStdout(), "Cron daemon: not running\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Job %q: cron_down\n", name)
				return nil
			}

			_, storeErr := store.Get(name)
			if storeErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cron daemon: running\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Job %q: unknown\n", name)
				return nil
			}

			job, err := client.Get(name)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cron daemon: running\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Job %q: missing\n", name)
				return nil
			}

			status := "active"
			if !job.Enabled {
				status = "disabled"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cron daemon: running\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Name:     %s\n", job.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Schedule: %s\n", job.Schedule)
			fmt.Fprintf(cmd.OutOrStdout(), "Command:  %s\n", job.Command)
			fmt.Fprintf(cmd.OutOrStdout(), "Status:   %s\n", status)
			return nil
		},
	}
}
