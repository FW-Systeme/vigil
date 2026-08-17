package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/FW-Systeme/Virgil/internal/process"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var appType string
	var port int
	var configFile string
	var force bool
	var entry string
	var buildDir string
	var workingDir string
	var envFile string
	var nginxDomain string
	var nginxPath string
	var nginxConfig string
	var command string
	var buildCmd string
	var smokeTestScript string
	var bundledDeps bool
	var installCmd string
	var killMode string

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Register a new app",
		Long: `Register a new application with vigil.

Examples:
  vigil add my-api --type node --entry /app/server.js --port 3000
  vigil add my-site --type static --build-dir /app/dist --port 8080
  vigil add --config ecosystem.json
  vigil add my-app --config ecosystem.json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}

			var nameArg string
			if len(args) > 0 {
				nameArg = args[0]
			}

			if configFile != "" {
				return addFromConfig(cmd, pm, configFile, nameArg, force)
			}

			if nameArg == "" {
				return fmt.Errorf("accepts 1 arg(s), received 0")
			}

			if appType == "" {
				return fmt.Errorf("required flag(s) \"type\" not set")
			}
			if port <= 0 {
				return fmt.Errorf("required flag(s) \"port\" not set")
			}

			return addSingle(cmd, pm, nameArg, appType, entry, buildDir, workingDir, envFile, nginxDomain, nginxPath, nginxConfig, command, buildCmd, port, force, smokeTestScript, bundledDeps, installCmd, killMode)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&appType, "type", "", "App type (node or static)")
	flags.IntVar(&port, "port", 0, "Port the app listens on")
	flags.StringVar(&entry, "entry", "", "Entry script (required for node apps)")
	flags.StringVar(&buildDir, "build-dir", "", "Build directory (required for static apps)")
	flags.StringVar(&workingDir, "working-dir", "", "Working directory for the app")
	flags.StringVar(&envFile, "env-file", "", "Path to environment file")
	flags.StringVar(&nginxDomain, "nginx-domain", "", "Nginx server_name (for static apps)")
	flags.StringVar(&nginxPath, "nginx-path", "", "Nginx root path (for static apps)")
	flags.StringVar(&nginxConfig, "nginx-config", "", "Path to custom nginx config file (for static apps, overrides auto-generation)")
	flags.StringVar(&configFile, "config", "", "Path to ecosystem JSON file")
	flags.BoolVar(&force, "force", false, "Overwrite existing app")
	flags.StringVar(&command, "command", "", "Custom ExecStart command (e.g. /opt/app/bin --port 3000)")
	flags.StringVar(&buildCmd, "build-cmd", "", "Build command to run before start (e.g. go build -o /opt/app/bin .)")
	flags.StringVar(&smokeTestScript, "smoke-test-script", "", "Path to smoke test script (activates release management)")
	flags.BoolVar(&bundledDeps, "bundled-deps", false, "Dependencies are included in the package")
	flags.StringVar(&installCmd, "install-cmd", "", "Dependency install command (e.g. 'yarn install --frozen-lockfile', 'pip install -r requirements.txt')")
	flags.StringVar(&killMode, "kill-mode", "", "systemd KillMode: process, control-group, mixed, or none (default: control-group)")

	return cmd
}

func addSingle(cmd *cobra.Command, pm *process.Manager, name, appType, entry, buildDir, workingDir, envFile, nginxDomain, nginxPath, nginxConfig, command, buildCmd string, port int, force bool, smokeTestScript string, bundledDeps bool, installCmd string, killMode string) error {
	p := process.Process{
		Name:            name,
		Type:            process.Type(appType),
		Port:            port,
		Entry:           entry,
		BuildDir:        buildDir,
		WorkingDir:      workingDir,
		EnvFile:         envFile,
		NginxDomain:     nginxDomain,
		NginxPath:       nginxPath,
		NginxConfig:     nginxConfig,
		Command:         command,
		BuildCmd:        buildCmd,
		CreatedAt:       time.Now(),
		Enabled:         true,
		SmokeTestScript: smokeTestScript,
		BundledDeps:     bundledDeps,
		InstallCmd:      installCmd,
		KillMode:        killMode,
	}

	if err := p.Validate(); err != nil {
		return err
	}

	if err := pm.AddProcess(cmd.Context(), p, force); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Registered app %q\n", name)
	return nil
}

func addFromConfig(cmd *cobra.Command, pm *process.Manager, path, filterName string, force bool) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening ecosystem file: %w", err)
	}
	defer f.Close()

	apps, err := process.ParseEcosystemFile(f)
	if err != nil {
		return err
	}

	if filterName != "" {
		found := false
		for _, app := range apps {
			if app.Name == filterName {
				apps = []process.Process{app}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("app %q not found in ecosystem file", filterName)
		}
	}

	var registered, errors int
	for _, app := range apps {
		if err := app.Validate(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skipping %q: %v\n", app.Name, err)
			errors++
			continue
		}

		if err := pm.AddProcess(cmd.Context(), app, force); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %q: %v\n", app.Name, err)
			errors++
			continue
		}
		registered++
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%d app(s) registered, %d error(s)\n", registered, errors)

	if errors > 0 {
		return fmt.Errorf("%d error(s) occurred", errors)
	}
	return nil
}
