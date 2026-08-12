package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FW-Systeme/Virgil/internal/cron"
	"github.com/FW-Systeme/Virgil/internal/nginx"
	"github.com/FW-Systeme/Virgil/internal/process"
	"github.com/FW-Systeme/Virgil/internal/systemd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSystemd struct {
	systemd.Client
	startCalled bool
	stopCalled  bool
	err         error
}

func (m *mockSystemd) StartUnit(ctx context.Context, name string) error {
	m.startCalled = true
	return m.err
}
func (m *mockSystemd) StopUnit(ctx context.Context, name string) error {
	m.stopCalled = true
	return m.err
}
func (m *mockSystemd) RestartUnit(ctx context.Context, name string) error {
	return m.err
}
func (m *mockSystemd) EnableUnit(ctx context.Context, name string) error { return m.err }
func (m *mockSystemd) DisableUnit(ctx context.Context, name string) error { return m.err }
func (m *mockSystemd) UnitStatus(ctx context.Context, name string) (string, string, error) {
	return "active", "running", m.err
}
func (m *mockSystemd) CreateUnitFile(name string, content []byte) error { return m.err }
func (m *mockSystemd) RemoveUnitFile(name string) error                 { return m.err }
func (m *mockSystemd) Reload(ctx context.Context) error                 { return m.err }
func (m *mockSystemd) Close() error                                    { return nil }
func (m *mockSystemd) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockSystemd) SetupLogging(ctx context.Context, name string, logPath string, maxSize string, rotate int) error {
	return m.err
}
func (m *mockSystemd) RemoveLogging(ctx context.Context, name string) error {
	return m.err
}

type mockNginx struct {
	nginx.Client
	err error
}

func (m *mockNginx) EnableSite(name string, port int, domain, root string) error { return m.err }
func (m *mockNginx) DisableSite(name string) error                               { return m.err }
func (m *mockNginx) RemoveSiteConfig(name string) error                          { return m.err }
func (m *mockNginx) SiteEnabled(name string) (bool, error)                       { return true, m.err }
func (m *mockNginx) Reload(ctx context.Context) error                            { return m.err }
func (m *mockNginx) Close() error                                                { return nil }
func (m *mockNginx) LogFile(name string) string { return "" }
func (m *mockNginx) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockNginx) SetupLogging(name string, logPath string, maxSize string, rotate int) error {
	return m.err
}
func (m *mockNginx) RemoveLogging(name string) error {
	return m.err
}

type mockStore struct {
	process.Store
	processes map[string]process.Process
	err       error
}

func (m *mockStore) Load(name string) (process.Process, error) {
	p, ok := m.processes[name]
	if !ok {
		return process.Process{}, fmt.Errorf("not found")
	}
	return p, nil
}
func (m *mockStore) Save(p process.Process) error  { return m.err }
func (m *mockStore) Delete(name string) error       { return m.err }
func (m *mockStore) List() ([]process.Process, error) {
	var list []process.Process
	for _, p := range m.processes {
		list = append(list, p)
	}
	return list, m.err
}

func testPM() *process.Manager {
	store := &mockStore{processes: map[string]process.Process{}}
	sd := &mockSystemd{}
	ng := &mockNginx{}
	return process.New(store, sd, ng)
}

func testPMWithProcesses(procs map[string]process.Process) *process.Manager {
	store := &mockStore{processes: procs}
	return process.New(store, &mockSystemd{}, &mockNginx{})
}

func executeWithPM(t *testing.T, pm *process.Manager, args []string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SetContext(pmCtx(cmd.Context(), pm))
		return nil
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if args != nil {
		cmd.SetArgs(args)
	}
	err := cmd.Execute()
	return buf.String(), err
}

func TestIsHelpOrVersion(t *testing.T) {
	root := &cobra.Command{Use: "vigil"}
	list := &cobra.Command{Use: "list"}
	versionCmd := &cobra.Command{Use: "version"}
	cron := &cobra.Command{Use: "cron"}
	cronAdd := &cobra.Command{Use: "add"}
	start := &cobra.Command{Use: "start"}

	root.AddCommand(list, versionCmd, cron, start)
	cron.AddCommand(cronAdd)

	tests := []struct {
		cmd  *cobra.Command
		want bool
	}{
		{root, true},
		{versionCmd, true},
		{list, false},
		{cron, false},
		{cronAdd, false},
		{start, false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd.CommandPath(), func(t *testing.T) {
			assert.Equal(t, tt.want, isHelpOrVersion(tt.cmd))
		})
	}
}

func TestNeedsSystemd(t *testing.T) {
	root := &cobra.Command{Use: "vigil"}
	list := &cobra.Command{Use: "list"}
	initCmd := &cobra.Command{Use: "init"}
	versionCmd := &cobra.Command{Use: "version"}
	cron := &cobra.Command{Use: "cron"}
	cronAdd := &cobra.Command{Use: "add"}
	add := &cobra.Command{Use: "add"}
	start := &cobra.Command{Use: "start"}
	stop := &cobra.Command{Use: "stop"}
	restart := &cobra.Command{Use: "restart"}
	remove := &cobra.Command{Use: "remove"}
	logs := &cobra.Command{Use: "logs"}
	update := &cobra.Command{Use: "update"}

	root.AddCommand(list, initCmd, versionCmd, cron, add, start, stop, restart, remove, logs, update)
	cron.AddCommand(cronAdd)

	tests := []struct {
		cmd  *cobra.Command
		want bool
	}{
		{root, false},
		{list, false},
		{initCmd, false},
		{versionCmd, false},
		{cron, false},
		{cronAdd, false},
		{add, true},
		{start, true},
		{stop, true},
		{restart, true},
		{remove, true},
		{logs, true},
		{update, true},
	}
	for _, tt := range tests {
		t.Run(tt.cmd.CommandPath(), func(t *testing.T) {
			assert.Equal(t, tt.want, needsSystemd(tt.cmd))
		})
	}
}

func TestExecute(t *testing.T) {
	SetVersion("test")
	err := Execute()
	require.NoError(t, err)
}

func TestExecuteWithVersion(t *testing.T) {
	SetVersion("1.2.3")
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "1.2.3\n", buf.String())
}

func TestHelp(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "Usage:")
	assert.Contains(t, out, "add")
	assert.Contains(t, out, "remove")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "start")
	assert.Contains(t, out, "stop")
	assert.Contains(t, out, "restart")
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "logs")
}

func TestVersion(t *testing.T) {
	SetVersion("1.0.0")
	out, err := executeWithPM(t, testPM(), []string{"version"})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0\n", out)
}

func TestNoArgsShowsHelp(t *testing.T) {
	out, err := executeWithPM(t, testPM(), nil)
	require.NoError(t, err)
	assert.Contains(t, out, "Usage:")
}

func TestUnknownCommand(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestAdd_NoArgs(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestAdd_MissingFlags(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestAdd_Success(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "node", "--entry", "./app.js", "--port", "3000", "--smoke-test-script", "/smoke.sh", "--install-cmd", "yarn install"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestRemove_NoArgs(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"remove"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestRemove_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"remove", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Removed")
}

func TestRemove_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"remove", "ghost"})
	require.Error(t, err)
}

func TestList_Empty(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "No apps")
}

func TestList_WithApps(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"alpha": {Name: "alpha", Type: process.TypeNode, Port: 3000, Enabled: true},
		"beta":  {Name: "beta", Type: process.TypeStatic, Port: 8080, Enabled: true},
	})
	out, err := executeWithPM(t, pm, []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")
}

func TestStart_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"start", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Started")
}

func TestStart_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"start", "ghost"})
	require.Error(t, err)
}

func TestStop_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"stop", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Stopped")
}

func TestStop_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"stop", "ghost"})
	require.Error(t, err)
}

func TestRestart_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"restart", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Restarted")
}

func TestRestart_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"restart", "ghost"})
	require.Error(t, err)
}

func TestStartCmd_NoPM(t *testing.T) {
	cmd := newStartCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestStopCmd_NoPM(t *testing.T) {
	cmd := newStopCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRestartCmd_NoPM(t *testing.T) {
	cmd := newRestartCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRemoveCmd_NoPM(t *testing.T) {
	cmd := newRemoveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestListCmd_NoPM(t *testing.T) {
	cmd := newListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	require.Error(t, err)
}

func TestInitCmd_CreatesFile(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"init", "--output", t.TempDir() + "/eco.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
}

func TestContextHelpers(t *testing.T) {
	pm := testPM()
	ctx := pmCtx(context.Background(), pm)
	got, ok := pmFromCtx(ctx)
	assert.True(t, ok)
	assert.Same(t, pm, got)

	_, ok = pmFromCtx(context.Background())
	assert.False(t, ok)
}

func TestAdd_WithConfigFile_Single(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"node","entry":"./app.js","port":3000,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestAdd_WithConfigFile_Array(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"a1","type":"node","entry":"e1","port":3000,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"},{"name":"a2","type":"static","build_dir":"bd","port":8080,"smoke_test_script":"/smoke.sh"}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "2 app(s) registered")
}

func TestAdd_WithConfigFile_NameFilter(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"},{"name":"app2","type":"node","entry":"e2","port":3001,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "app1", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestAdd_WithConfigFile_NameFilterNotFound(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000}]}`), 0600)
	require.NoError(t, err)
	_, err = executeWithPM(t, testPM(), []string{"add", "unknown", "--config", ecoFile})
	require.Error(t, err)
}

func TestAdd_WithConfigFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte("{broken"), 0600)
	require.NoError(t, err)
	_, err = executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.Error(t, err)
}

func TestAdd_Static(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--build-dir", "./dist", "--port", "8080", "--smoke-test-script", "/smoke.sh"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithConfigAndNameArg(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"},{"name":"app2","type":"node","entry":"e2","port":3001,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "app1", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestStart_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"start"})
	require.Error(t, err)
}

func TestStop_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"stop"})
	require.Error(t, err)
}

func TestRestart_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"restart"})
	require.Error(t, err)
}

func TestAdd_WithFlagsOnly(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-api", "--type", "node", "--port", "3000", "--entry", "app.js", "--smoke-test-script", "/smoke.sh", "--install-cmd", "npm ci"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_StaticWithEntry(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--build-dir", "./dist", "--port", "8080", "--entry", "ignored.js", "--smoke-test-script", "/smoke.sh"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithInvalidType(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "invalid", "--port", "3000"})
	require.Error(t, err)
}

func TestAdd_WithMissingBuildDir(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--port", "8080"})
	require.Error(t, err)
}

func TestAdd_WithMissingEntry(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "node", "--port", "3000"})
	require.Error(t, err)
}

func TestAdd_ConfigAndNameArgTooMany(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "a", "b", "--config", "eco.json"})
	require.Error(t, err)
}

func TestInitCmd_HelpShowsFlag(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"init", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--output")
}

func TestInitCmd_CustomOutput(t *testing.T) {
	dir := t.TempDir()
	out, err := executeWithPM(t, testPM(), []string{"init", "--output", dir + "/test-eco.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
	_, err = os.Stat(dir + "/test-eco.json")
	require.NoError(t, err)
}

func TestCommandFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOut string
	}{
		{"add help shows type flag", []string{"add", "--help"}, "--type"},
		{"add help shows port flag", []string{"add", "--help"}, "--port"},
		{"remove help shows usage", []string{"remove", "--help"}, "remove <name>"},
		{"list help shows usage", []string{"list", "--help"}, "list"},
		{"start help shows usage", []string{"start", "--help"}, "start"},
		{"stop help shows usage", []string{"stop", "--help"}, "stop"},
		{"restart help shows usage", []string{"restart", "--help"}, "restart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeWithPM(t, testPM(), tt.args)
			require.NoError(t, err)
			assert.Contains(t, out, tt.wantOut)
		})
	}
}

func TestAdd_DuplicateWithoutForce(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode, SmokeTestScript: "/smoke.sh"},
	})
	out, err := executeWithPM(t, pm, []string{"add", "my-app", "--type", "node", "--entry", "./app.js", "--port", "3000", "--smoke-test-script", "/smoke.sh", "--install-cmd", "yarn install"})
	require.Error(t, err)
	assert.Contains(t, out, "already exists")
}

func TestAdd_DuplicateWithForce(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode, SmokeTestScript: "/smoke.sh"},
	})
	out, err := executeWithPM(t, pm, []string{"add", "my-app", "--type", "static", "--build-dir", "./dist", "--port", "8080", "--force", "--smoke-test-script", "/smoke.sh"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithConfigFile_Duplicate(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"node","entry":"./app.js","port":3000,"smoke_test_script":"/smoke.sh","install_cmd":"yarn install"}`), 0600)
	require.NoError(t, err)

	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode, SmokeTestScript: "/smoke.sh"},
	})
	out, err := executeWithPM(t, pm, []string{"add", "--config", ecoFile})
	require.Error(t, err)
	assert.Contains(t, out, "already exists")
}

func TestAdd_WithConfigFile_Force(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"static","build_dir":"./dist","port":8080,"smoke_test_script":"/smoke.sh"}`), 0600)
	require.NoError(t, err)

	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"add", "--config", ecoFile, "--force"})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestList_WithDisabledApp(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"alpha": {Name: "alpha", Type: process.TypeNode, Port: 3000, Enabled: false},
	})
	out, err := executeWithPM(t, pm, []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "disabled")
}

func TestLogs_Help(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"logs", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--lines")
	assert.Contains(t, out, "--follow")
	assert.Contains(t, out, "--output")
}

func TestLogs_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"logs"})
	require.Error(t, err)
}

func TestLogs_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"logs", "my-app"})
	require.NoError(t, err)
	// No error, output is empty since mock returns empty reader
	assert.Empty(t, out)
}

func TestLogs_NoPM(t *testing.T) {
	cmd := newLogsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestLogs_OutputFlag(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.log")

	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"logs", "my-app", "--output", outputPath})
	require.NoError(t, err)
	assert.Empty(t, out)
}

// --- Cron command tests ---

type mockCronStore struct {
	cron.Store
	jobs map[string]cron.Job
	err  error
}

func (m *mockCronStore) Get(name string) (cron.Job, error) {
	if m.err != nil {
		return cron.Job{}, m.err
	}
	j, ok := m.jobs[name]
	if !ok {
		return cron.Job{}, cron.ErrNotFound
	}
	return j, nil
}

func (m *mockCronStore) Save(job cron.Job) error {
	if m.err != nil {
		return m.err
	}
	m.jobs[job.Name] = job
	return nil
}

func (m *mockCronStore) Delete(name string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.jobs, name)
	return nil
}

func (m *mockCronStore) List() ([]cron.Job, error) {
	if m.err != nil {
		return nil, m.err
	}
	var list []cron.Job
	for _, j := range m.jobs {
		list = append(list, j)
	}
	return list, nil
}

func newMockCronStore() *mockCronStore {
	return &mockCronStore{jobs: make(map[string]cron.Job)}
}

type mockCronClient struct {
	cron.Client
	jobs         map[string]cron.Job
	cronRunning  bool
	installErr   error
	removeErr    error
	enableErr    error
	disableErr   error
}

func newMockCronClient() *mockCronClient {
	return &mockCronClient{jobs: make(map[string]cron.Job), cronRunning: true}
}

func (m *mockCronClient) Install(job cron.Job) error {
	if m.installErr != nil {
		return m.installErr
	}
	if _, exists := m.jobs[job.Name]; exists {
		return cron.ErrAlreadyExists
	}
	m.jobs[job.Name] = job
	return nil
}

func (m *mockCronClient) Remove(name string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	if _, exists := m.jobs[name]; !exists {
		return cron.ErrNotFound
	}
	delete(m.jobs, name)
	return nil
}

func (m *mockCronClient) Enable(name string) error {
	if m.enableErr != nil {
		return m.enableErr
	}
	job, exists := m.jobs[name]
	if !exists {
		return cron.ErrNotFound
	}
	job.Enabled = true
	m.jobs[name] = job
	return nil
}

func (m *mockCronClient) Disable(name string) error {
	if m.disableErr != nil {
		return m.disableErr
	}
	job, exists := m.jobs[name]
	if !exists {
		return cron.ErrNotFound
	}
	job.Enabled = false
	m.jobs[name] = job
	return nil
}

func (m *mockCronClient) List() ([]cron.Job, error) {
	var list []cron.Job
	for _, j := range m.jobs {
		list = append(list, j)
	}
	return list, nil
}

func (m *mockCronClient) Get(name string) (cron.Job, error) {
	job, exists := m.jobs[name]
	if !exists {
		return cron.Job{}, cron.ErrNotFound
	}
	return job, nil
}

func (m *mockCronClient) IsCronRunning() (bool, error) {
	return m.cronRunning, nil
}

func TestCronAdd_NoArgs(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "add"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestCronAdd_MissingSchedule(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "add", "myjob", "--command", "/bin/test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestCronAdd_MissingCommand(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "add", "myjob", "--schedule", "0 3 * * *"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestCronAdd_Success(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myjob", "--schedule", "0 3 * * *", "--command", "/bin/test"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Added")
	assert.Contains(t, buf.String(), "myjob")

	job, err := cs.Get("myjob")
	require.NoError(t, err)
	assert.Equal(t, "0 3 * * *", job.Schedule)
}

func TestCronAdd_Duplicate(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)
	require.NoError(t, cs.Save(cron.Job{Name: "dup"}))

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"dup", "--schedule", "0 3 * * *", "--command", "/bin/test"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}



func TestCronInit_CreatesFile(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"cron", "init", "--output", t.TempDir() + "/cron.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
}

func TestCronInit_CustomOutput(t *testing.T) {
	dir := t.TempDir()
	out, err := executeWithPM(t, testPM(), []string{"cron", "init", "--output", dir + "/my-cron.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
	_, err = os.Stat(dir + "/my-cron.json")
	require.NoError(t, err)
}

func TestCronList_Empty(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No cron jobs configured")
}

func TestCronList_WithJobs(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "alpha", Schedule: "0 1 * * *", Command: "/bin/a", Enabled: true}))
	require.NoError(t, cs.Save(cron.Job{Name: "beta", Schedule: "0 2 * * *", Command: "/bin/b", Enabled: false}))

	cmd := newCronListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "alpha")
	assert.Contains(t, buf.String(), "beta")
	assert.Contains(t, buf.String(), "disabled")
}

func TestCronRemove_Success(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "rm-me", Schedule: "0 3 * * *", Command: "/bin/rm"}))
	require.NoError(t, cc.Install(cron.Job{Name: "rm-me", Schedule: "0 3 * * *", Command: "/bin/rm"}))

	cmd := newCronRemoveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"rm-me"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Removed")

	_, err = cs.Get("rm-me")
	require.Error(t, err)
}

func TestCronRemove_NotFound(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronRemoveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"ghost"})
	err := cmd.Execute()
	require.NoError(t, err) // store.Delete ignores not-exist
}

func TestCronEnable_Success(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: false}))
	require.NoError(t, cc.Install(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: false}))

	cmd := newCronEnableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myjob"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Enabled")

	job, _ := cs.Get("myjob")
	assert.True(t, job.Enabled)
}

func TestCronEnable_NotFound(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronEnableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"ghost"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestCronDisable_Success(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: true}))
	require.NoError(t, cc.Install(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: true}))

	cmd := newCronDisableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myjob"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Disabled")

	job, _ := cs.Get("myjob")
	assert.False(t, job.Enabled)
}

func TestCronDisable_NotFound(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronDisableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"ghost"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestCronStatus_Active(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: true}))
	require.NoError(t, cc.Install(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: true}))

	cmd := newCronStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myjob"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "active")
	assert.Contains(t, buf.String(), "Cron daemon: running")
}

func TestCronStatus_CronDown(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	cc.cronRunning = false
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "myjob", Schedule: "0 3 * * *", Command: "/bin/test", Enabled: true}))

	cmd := newCronStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myjob"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "cron_down")
}

func TestCronStatus_Unknown(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"unknown"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "unknown")
}

func TestCronStatus_Missing(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	require.NoError(t, cs.Save(cron.Job{Name: "orphan", Schedule: "0 3 * * *", Command: "/bin/orphan", Enabled: true}))
	// Do NOT install in crontab

	cmd := newCronStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"orphan"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "missing")
}

func TestCronAdd_WithConfigFile_Single(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"name":"myjob","schedule":"0 3 * * *","command":"/bin/test"}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "1 cron job(s) added")
}

func TestCronAdd_WithConfigFile_Array(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"crons":[{"name":"a","schedule":"0 1 * * *","command":"cmd1"},{"name":"b","schedule":"0 2 * * *","command":"cmd2"}]}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "2 cron job(s) added")
}

func TestCronAdd_WithConfigFile_NameFilter(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"crons":[{"name":"app1","schedule":"0 1 * * *","command":"cmd1"},{"name":"app2","schedule":"0 2 * * *","command":"cmd2"}]}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"app1", "--config", cfgFile})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "1 cron job(s) added")
}

func TestCronAdd_WithConfigFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte("{broken"), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.Error(t, err)
}

func TestCronHelp(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"cron", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "init")
	assert.Contains(t, out, "add")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "remove")
	assert.Contains(t, out, "enable")
	assert.Contains(t, out, "disable")
	assert.Contains(t, out, "status")
}

func TestCronInit_HelpShowsFlag(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"cron", "init", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--output")
}

func TestCronStatus_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "status"})
	require.Error(t, err)
}

func TestCronRemove_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "remove"})
	require.Error(t, err)
}

func TestCronEnable_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "enable"})
	require.Error(t, err)
}

func TestCronDisable_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"cron", "disable"})
	require.Error(t, err)
}

func TestCronCtx_Empty(t *testing.T) {
	_, _, ok := cronFromCtx(context.Background())
	assert.False(t, ok)
}

func TestCronCtx_Roundtrip(t *testing.T) {
	cs := newMockCronStore()
	cc := newMockCronClient()
	ctx := cronCtx(context.Background(), cs, cc)
	gotStore, gotClient, ok := cronFromCtx(ctx)
	assert.True(t, ok)
	assert.Same(t, cs, gotStore)
	assert.Same(t, cc, gotClient)
}

func TestAddCronFromConfig_InvalidJobInConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"crons":[{"name":"","schedule":"0 1 * * *","command":"cmd1"}]}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "Warning")
}

func TestAddCronFromConfig_DuplicateInConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"name":"dup","schedule":"0 1 * * *","command":"cmd1"}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	require.NoError(t, cs.Save(cron.Job{Name: "dup", Schedule: "0 1 * * *", Command: "cmd1"}))
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "already exists")
}

func TestCronInit_DefaultOutput(t *testing.T) {
	dir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origWd) }()

	cmd := newCronInitCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "cronfile.json")
	_, err = os.Stat("cronfile.json")
	require.NoError(t, err)
	os.Remove("cronfile.json")
}

func TestAddCronFromConfig_InstallError(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"name":"failjob","schedule":"0 1 * * *","command":"cmd1"}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	cc.installErr = fmt.Errorf("crontab unavailable")
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--config", cfgFile})
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "Error installing")
}

func TestCronAdd_WithConfigFile_NameFilterNotFound(t *testing.T) {
	dir := t.TempDir()
	cfgFile := dir + "/cron.json"
	err := os.WriteFile(cfgFile, []byte(`{"crons":[{"name":"app1","schedule":"0 1 * * *","command":"cmd1"}]}`), 0600)
	require.NoError(t, err)

	cs := newMockCronStore()
	cc := newMockCronClient()
	pm := testPM()
	ctx := cronCtx(pmCtx(context.Background(), pm), cs, cc)

	cmd := newCronAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"unknown", "--config", cfgFile})
	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "0 cron job(s) added")
}

// --- Log output tests ---

func TestOpenLogFile_Directory(t *testing.T) {
	dir := t.TempDir()
	f, err := openLogFile(dir, "vigil start")
	require.NoError(t, err)
	defer f.Close()

	name := f.Name()
	assert.DirExists(t, filepath.Dir(name))
	assert.Contains(t, name, "vigil-start-")
	assert.True(t, strings.HasSuffix(name, ".log"))
}

func TestOpenLogFile_File(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "my-debug.log")

	f, err := openLogFile(logPath, "vigil update")
	require.NoError(t, err)
	defer f.Close()

	assert.Equal(t, logPath, f.Name())
	assert.FileExists(t, logPath)
}

func TestOpenLogFile_CreatesParent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sub", "nested", "my-debug.log")

	f, err := openLogFile(logPath, "vigil start")
	require.NoError(t, err)
	defer f.Close()

	assert.Equal(t, logPath, f.Name())
	assert.FileExists(t, logPath)
}

func TestOpenLogFile_TrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "logs") + string(os.PathSeparator)

	f, err := openLogFile(subDir, "vigil cron add")
	require.NoError(t, err)
	defer f.Close()

	name := f.Name()
	assert.Contains(t, name, "vigil-cron-add-")
	assert.True(t, strings.HasSuffix(name, ".log"))
	assert.DirExists(t, filepath.Dir(name))
}

func TestLogOutput_Integration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	SetVersion("1.2.3-test")

	logPath := filepath.Join(dir, "version.log")
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version", "--log-output", logPath})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "1.2.3-test\n", buf.String())

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "1.2.3-test")
}

func TestLogOutput_Appends(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	SetVersion("v1")
	logPath := filepath.Join(dir, "append.log")

	cmd1 := NewRootCmd()
	buf1 := new(bytes.Buffer)
	cmd1.SetOut(buf1)
	cmd1.SetErr(buf1)
	cmd1.SetArgs([]string{"version", "--log-output", logPath})
	err := cmd1.Execute()
	require.NoError(t, err)

	cmd2 := NewRootCmd()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"version", "--log-output", logPath})
	err = cmd2.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(data), "v1"))
}
