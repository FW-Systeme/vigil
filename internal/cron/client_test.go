package cron

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient implements Client in-memory for integration testing
// without depending on os/exec or system crontab.
type fakeClient struct {
	jobs map[string]Job // name -> Job
}

func newFakeClient() *fakeClient {
	return &fakeClient{jobs: make(map[string]Job)}
}

func (c *fakeClient) Install(job Job) error {
	if err := job.Validate(); err != nil {
		return err
	}
	if _, exists := c.jobs[job.Name]; exists {
		return ErrAlreadyExists
	}
	c.jobs[job.Name] = job
	return nil
}

func (c *fakeClient) Remove(name string) error {
	if _, exists := c.jobs[name]; !exists {
		return ErrNotFound
	}
	delete(c.jobs, name)
	return nil
}

func (c *fakeClient) Enable(name string) error {
	job, exists := c.jobs[name]
	if !exists {
		return ErrNotFound
	}
	job.Enabled = true
	c.jobs[name] = job
	return nil
}

func (c *fakeClient) Disable(name string) error {
	job, exists := c.jobs[name]
	if !exists {
		return ErrNotFound
	}
	job.Enabled = false
	c.jobs[name] = job
	return nil
}

func (c *fakeClient) List() ([]Job, error) {
	result := make([]Job, 0, len(c.jobs))
	for _, j := range c.jobs {
		result = append(result, j)
	}
	return result, nil
}

func (c *fakeClient) Get(name string) (Job, error) {
	job, exists := c.jobs[name]
	if !exists {
		return Job{}, ErrNotFound
	}
	return job, nil
}

func (c *fakeClient) IsCronRunning() (bool, error) {
	return true, nil
}

// compile-time check: fakeClient satisfies Client
var _ Client = (*fakeClient)(nil)

// --- Client interface integration tests (in-memory fake) ---

func TestClient_Install(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	err := client.Install(Job{Name: "backup", Schedule: "0 3 * * *", Command: "/usr/bin/backup"})
	require.NoError(t, err)

	got, err := client.Get("backup")
	require.NoError(t, err)
	assert.Equal(t, "backup", got.Name)
	assert.Equal(t, "0 3 * * *", got.Schedule)
	assert.Equal(t, "/usr/bin/backup", got.Command)
}

func TestClient_InstallDuplicate(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "dupe", Schedule: "0 1 * * *", Command: "/bin/true"}))
	err := client.Install(Job{Name: "dupe", Schedule: "0 2 * * *", Command: "/bin/other"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAlreadyExists)
}

func TestClient_Install_InvalidJob(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	err := client.Install(Job{Name: "", Schedule: "", Command: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestClient_Remove(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "cleanup", Schedule: "0 4 * * *", Command: "/opt/cleanup"}))

	err := client.Remove("cleanup")
	require.NoError(t, err)

	_, err = client.Get("cleanup")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestClient_RemoveNotFound(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	err := client.Remove("nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestClient_Enable(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "toggle", Schedule: "0 5 * * *", Command: "/bin/true"}))

	err := client.Disable("toggle")
	require.NoError(t, err)

	err = client.Enable("toggle")
	require.NoError(t, err)

	got, err := client.Get("toggle")
	require.NoError(t, err)
	assert.True(t, got.Enabled)
}

func TestClient_EnableNotFound(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	err := client.Enable("ghost")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestClient_Enable_AlreadyEnabled(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "enabled-job", Schedule: "0 6 * * *", Command: "/bin/true", Enabled: true}))

	err := client.Enable("enabled-job")
	require.NoError(t, err)
}

func TestClient_Disable(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "toggle", Schedule: "0 5 * * *", Command: "/bin/true"}))

	err := client.Disable("toggle")
	require.NoError(t, err)

	got, err := client.Get("toggle")
	require.NoError(t, err)
	assert.False(t, got.Enabled)
}

func TestClient_DisableNotFound(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	err := client.Disable("ghost")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestClient_Disable_AlreadyDisabled(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "disable-me", Schedule: "0 7 * * *", Command: "/bin/true", Enabled: false}))

	err := client.Disable("disable-me")
	require.NoError(t, err)
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "a", Schedule: "0 1 * * *", Command: "c1"}))
	require.NoError(t, client.Install(Job{Name: "b", Schedule: "0 2 * * *", Command: "c2"}))

	jobs, err := client.List()
	require.NoError(t, err)
	require.Len(t, jobs, 2)
}

func TestClient_ListEmpty(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	jobs, err := client.List()
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	job := Job{Name: "myjob", Schedule: "*/5 * * * *", Command: "/usr/bin/healthcheck", Enabled: true}
	require.NoError(t, client.Install(job))

	got, err := client.Get("myjob")
	require.NoError(t, err)
	assert.Equal(t, job.Name, got.Name)
	assert.Equal(t, job.Schedule, got.Schedule)
	assert.Equal(t, job.Command, got.Command)
	assert.True(t, got.Enabled)
}

func TestClient_GetNotFound(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	_, err := client.Get("missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestClient_InstallAndDisableCycle(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "x", Schedule: "0 0 * * *", Command: "/bin/x"}))
	require.NoError(t, client.Disable("x"))

	got, _ := client.Get("x")
	assert.False(t, got.Enabled)

	require.NoError(t, client.Enable("x"))

	got, _ = client.Get("x")
	assert.True(t, got.Enabled)
}

func TestClient_RemoveUpdatesList(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "keep", Schedule: "0 1 * * *", Command: "/bin/keep"}))
	require.NoError(t, client.Install(Job{Name: "remove", Schedule: "0 2 * * *", Command: "/bin/remove"}))

	require.NoError(t, client.Remove("remove"))

	jobs, err := client.List()
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "keep", jobs[0].Name)
}

func TestClientMultipleJobs(t *testing.T) {
	t.Parallel()
	client := newFakeClient()

	require.NoError(t, client.Install(Job{Name: "first", Schedule: "0 1 * * *", Command: "/bin/first", Enabled: true}))
	require.NoError(t, client.Install(Job{Name: "second", Schedule: "0 2 * * *", Command: "/bin/second", Enabled: true}))
	require.NoError(t, client.Install(Job{Name: "third", Schedule: "0 3 * * *", Command: "/bin/third", Enabled: true}))

	jobs, err := client.List()
	require.NoError(t, err)
	require.Len(t, jobs, 3)

	require.NoError(t, client.Remove("second"))

	jobs, err = client.List()
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	require.NoError(t, client.Disable("first"))

	got, err := client.Get("first")
	require.NoError(t, err)
	assert.False(t, got.Enabled)

	got, err = client.Get("third")
	require.NoError(t, err)
	assert.True(t, got.Enabled)
}

// --- Unit tests for internal helper functions (real implementations) ---

func TestMarkerLine(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "# vigil-cron: my-job", markerLine("my-job"))
	assert.Equal(t, "# vigil-cron: ", markerLine(""))
}

func TestParseEntryLine_Valid(t *testing.T) {
	t.Parallel()
	sched, cmd := parseEntryLine("0 3 * * * /usr/bin/backup --flag")
	assert.Equal(t, "0 3 * * *", sched)
	assert.Equal(t, "/usr/bin/backup --flag", cmd)
}

func TestParseEntryLine_ShortLine(t *testing.T) {
	t.Parallel()
	sched, cmd := parseEntryLine("0 3 * *")
	assert.Equal(t, "", sched)
	assert.Equal(t, "", cmd)
}

func TestParseEntryLine_Empty(t *testing.T) {
	t.Parallel()
	sched, cmd := parseEntryLine("")
	assert.Equal(t, "", sched)
	assert.Equal(t, "", cmd)
}

func TestFindEntry_Found(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: test", "0 5 * * * /bin/test"}
	idx, disabled, found := findEntry(lines, "test")
	assert.True(t, found)
	assert.Equal(t, 1, idx)
	assert.False(t, disabled)
}

func TestFindEntry_Disabled(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: test", "#DISABLED# 0 5 * * * /bin/test"}
	idx, disabled, found := findEntry(lines, "test")
	assert.True(t, found)
	assert.Equal(t, 1, idx)
	assert.True(t, disabled)
}

func TestFindEntry_NotFound(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: other", "0 5 * * * /bin/other"}
	_, _, found := findEntry(lines, "missing")
	assert.False(t, found)
}

func TestFindEntry_NoEntryLine(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: orphan"}
	_, _, found := findEntry(lines, "orphan")
	assert.False(t, found)
}

func TestFindEntry_MultipleMarkers(t *testing.T) {
	t.Parallel()
	lines := []string{
		"# vigil-cron: first", "0 1 * * * /bin/first",
		"# vigil-cron: second", "0 2 * * * /bin/second",
	}
	idx, disabled, found := findEntry(lines, "second")
	assert.True(t, found)
	assert.Equal(t, 3, idx)
	assert.False(t, disabled)

	idx, disabled, found = findEntry(lines, "first")
	assert.True(t, found)
	assert.Equal(t, 1, idx)
}

func TestFindEntry_CaseSensitive(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: MyJob", "0 5 * * * /bin/MyJob"}
	_, _, found := findEntry(lines, "myjob")
	assert.False(t, found)
}

func TestExtractJobs_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, extractJobs([]string{}))
	assert.Empty(t, extractJobs([]string{"# comment", "0 5 * * * /bin/true"}))
}

func TestExtractJobs_Single(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: backup", "0 3 * * * /usr/bin/backup"}
	jobs := extractJobs(lines)
	require.Len(t, jobs, 1)
	assert.Equal(t, "backup", jobs[0].Name)
	assert.Equal(t, "0 3 * * *", jobs[0].Schedule)
	assert.Equal(t, "/usr/bin/backup", jobs[0].Command)
	assert.True(t, jobs[0].Enabled)
}

func TestExtractJobs_Disabled(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: paused", "#DISABLED# 0 4 * * * /bin/paused"}
	jobs := extractJobs(lines)
	require.Len(t, jobs, 1)
	assert.Equal(t, "paused", jobs[0].Name)
	assert.False(t, jobs[0].Enabled)
}

func TestExtractJobs_Multiple(t *testing.T) {
	t.Parallel()
	lines := []string{
		"# vigil-cron: a", "0 1 * * * /bin/a",
		"# vigil-cron: b", "#DISABLED# 0 2 * * * /bin/b",
		"# vigil-cron: c", "0 3 * * * /bin/c",
	}
	jobs := extractJobs(lines)
	require.Len(t, jobs, 3)
	assert.Equal(t, "a", jobs[0].Name)
	assert.True(t, jobs[0].Enabled)
	assert.Equal(t, "b", jobs[1].Name)
	assert.False(t, jobs[1].Enabled)
	assert.Equal(t, "c", jobs[2].Name)
	assert.True(t, jobs[2].Enabled)
}

func TestExtractJobs_IgnoresNonVigilEntries(t *testing.T) {
	t.Parallel()
	lines := []string{
		"# vigil-cron: myjob", "0 5 * * * /bin/myjob",
		"# regular cron entry", "30 2 * * * /usr/bin/cleanup",
	}
	jobs := extractJobs(lines)
	require.Len(t, jobs, 1)
	assert.Equal(t, "myjob", jobs[0].Name)
}

func TestExtractJobs_EmptyName(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron:", "0 5 * * * /usr/bin/noname"}
	assert.Empty(t, extractJobs(lines))
}

func TestExtractJobs_MissingEntryLine(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: orphan"}
	assert.Empty(t, extractJobs(lines))
}

func TestExtractJobs_InvalidSchedule(t *testing.T) {
	t.Parallel()
	lines := []string{"# vigil-cron: bad", "short /bin/bad"}
	assert.Empty(t, extractJobs(lines))
}

func TestExtractJobs_ExtraWhitespace(t *testing.T) {
	t.Parallel()
	lines := []string{"  # vigil-cron:  spaced-job  ", "  0 5 * * *  /bin/spaced  "}
	jobs := extractJobs(lines)
	require.Len(t, jobs, 1)
	assert.Equal(t, "spaced-job", jobs[0].Name)
	assert.Equal(t, "0 5 * * *", jobs[0].Schedule)
	assert.Equal(t, "/bin/spaced", jobs[0].Command)
}

// --- readLines / writeLines tests with mock execCommand ---

func TestReadLines_EmptyCrontab(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "exit 1")
		},
	}
	lines, err := client.readLines()
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestReadLines_Success(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "# test\n0 5 * * * /bin/test")
		},
	}
	lines, err := client.readLines()
	require.NoError(t, err)
	assert.Equal(t, []string{"# test", "0 5 * * * /bin/test"}, lines)
}

func TestReadLines_EmptyOutput(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "-n", "")
		},
	}
	lines, err := client.readLines()
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestWriteLines_Success(t *testing.T) {
	var receivedStdin string
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			cmd := exec.Command("sh", "-c", "cat")
			return cmd
		},
	}
	client.execCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command("sh", "-c", "cat > /dev/null")
		_ = receivedStdin
		return cmd
	}
	err := client.writeLines([]string{"# vigil-cron: test", "0 5 * * * /bin/test"})
	require.NoError(t, err)
}

func TestInstall_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			switch args[0] {
			case "-l":
				return exec.Command("sh", "-c", "exit 1")
			case "-":
				return exec.Command("sh", "-c", "cat > /dev/null")
			}
			return exec.Command("echo")
		},
	}
	err := client.Install(Job{Name: "mockjob", Schedule: "0 3 * * *", Command: "/bin/mock"})
	require.NoError(t, err)
}

func TestInstall_Duplicate_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: dup\n0 3 * * * /bin/dup\n")
			}
			return exec.Command("echo")
		},
	}
	err := client.Install(Job{Name: "dup", Schedule: "0 3 * * *", Command: "/bin/dup"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAlreadyExists)
}

func TestRemove_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: rml\n0 3 * * * /bin/rml\n")
			}
			if args[0] == "-" {
				return exec.Command("sh", "-c", "cat > /dev/null")
			}
			return exec.Command("echo")
		},
	}
	err := client.Remove("rml")
	require.NoError(t, err)
}

func TestEnable_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: en\n#DISABLED# 0 3 * * * /bin/en\n")
			}
			if args[0] == "-" {
				return exec.Command("sh", "-c", "cat > /dev/null")
			}
			return exec.Command("echo")
		},
	}
	err := client.Enable("en")
	require.NoError(t, err)
}

func TestDisable_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: dis\n0 3 * * * /bin/dis\n")
			}
			if args[0] == "-" {
				return exec.Command("sh", "-c", "cat > /dev/null")
			}
			return exec.Command("echo")
		},
	}
	err := client.Disable("dis")
	require.NoError(t, err)
}

func TestGet_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: getter\n0 3 * * * /bin/getter\n")
			}
			return exec.Command("echo")
		},
	}
	job, err := client.Get("getter")
	require.NoError(t, err)
	assert.Equal(t, "getter", job.Name)
	assert.Equal(t, "0 3 * * *", job.Schedule)
	assert.Equal(t, "/bin/getter", job.Command)
}

func TestGet_NotFound_WithMockExec(t *testing.T) {
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			if args[0] == "-l" {
				return exec.Command("printf", "# vigil-cron: other\n0 3 * * * /bin/other\n")
			}
			return exec.Command("echo")
		},
	}
	_, err := client.Get("missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- IsCronRunning test with mock execCommand ---

func TestIsCronRunning_UsesExecCommand(t *testing.T) {
	execCalled := false
	client := &cronClient{
		execCommand: func(name string, args ...string) *exec.Cmd {
			execCalled = true
			return exec.Command("echo", "active")
		},
	}
	running, err := client.IsCronRunning()
	require.NoError(t, err)
	assert.True(t, running)
	assert.True(t, execCalled)
}

// --- Compile checks ---

func TestCompileCheckStoreInterface(t *testing.T) {
	var store Store = &jsonFileStore{}
	_ = store
}

func TestCompileCheckClientInterface(t *testing.T) {
	var client Client = &cronClient{}
	_ = client
}

// --- Benchmark ---

func BenchmarkClient_InstallAndGet(b *testing.B) {
	client := newFakeClient()
	job := Job{Name: "bench", Schedule: "0 0 * * *", Command: "/bin/bench"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Install(job)
		_, _ = client.Get("bench")
		_ = client.Remove("bench")
	}
}
