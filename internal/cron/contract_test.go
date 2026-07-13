package cron

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobValidate_Valid(t *testing.T) {
	t.Parallel()
	j := Job{Name: "backup", Schedule: "0 3 * * *", Command: "/usr/bin/backup"}
	assert.NoError(t, j.Validate())
}

func TestJobValidate_MissingName(t *testing.T) {
	t.Parallel()
	j := Job{Name: "", Schedule: "0 3 * * *", Command: "/usr/bin/backup"}
	err := j.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestJobValidate_MissingSchedule(t *testing.T) {
	t.Parallel()
	j := Job{Name: "backup", Schedule: "", Command: "/usr/bin/backup"}
	err := j.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schedule is required")
}

func TestJobValidate_MissingCommand(t *testing.T) {
	t.Parallel()
	j := Job{Name: "backup", Schedule: "0 3 * * *", Command: ""}
	err := j.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestParseCronFile_SingleJob(t *testing.T) {
	t.Parallel()
	input := `{"name":"backup","schedule":"0 3 * * *","command":"/usr/bin/backup"}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "backup", jobs[0].Name)
	assert.Equal(t, "0 3 * * *", jobs[0].Schedule)
	assert.Equal(t, "/usr/bin/backup", jobs[0].Command)
}

func TestParseCronFile_MultipleJobs(t *testing.T) {
	t.Parallel()
	input := `{"crons":[{"name":"a","schedule":"0 1 * * *","command":"cmd1"},{"name":"b","schedule":"0 2 * * *","command":"cmd2"}]}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	assert.Equal(t, "a", jobs[0].Name)
	assert.Equal(t, "b", jobs[1].Name)
}

func TestParseCronFile_EmptyCronsArray(t *testing.T) {
	t.Parallel()
	input := `{"crons":[]}`
	_, err := ParseCronFile(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseCronFile_InvalidJSON(t *testing.T) {
	t.Parallel()
	input := `{broken`
	_, err := ParseCronFile(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestParseCronFile_ObjectWithoutName(t *testing.T) {
	t.Parallel()
	input := `{"schedule":"0 5 * * *","command":"/bin/true"}`
	_, err := ParseCronFile(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must contain either")
}

func TestParseCronFile_ExtraFields(t *testing.T) {
	t.Parallel()
	input := `{"name":"backup","schedule":"0 3 * * *","command":"/usr/bin/backup","extra":"ignored","foo":"bar"}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "backup", jobs[0].Name)
}

func TestParseCronFile_ExtraFieldsInArray(t *testing.T) {
	t.Parallel()
	input := `{"crons":[{"name":"a","schedule":"0 1 * * *","command":"cmd1","extra":"field"}]}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "a", jobs[0].Name)
}

func TestParseCronFile_WithEnabledAndCreatedAt(t *testing.T) {
	t.Parallel()
	// Job object with optional fields
	input := `{"name":"backup","schedule":"0 3 * * *","command":"/usr/bin/backup","enabled":true,"created_at":"2025-06-15T10:30:00Z"}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.True(t, jobs[0].Enabled)
	assert.Equal(t, time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC), jobs[0].CreatedAt)
}

func TestParseCronFile_SingleJobAllFields(t *testing.T) {
	t.Parallel()
	input := `{"name":"cleanup","schedule":"0 4 * * 0","command":"/opt/cleanup.sh","enabled":false,"created_at":"2025-01-01T00:00:00Z"}`
	jobs, err := ParseCronFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "cleanup", jobs[0].Name)
	assert.Equal(t, "0 4 * * 0", jobs[0].Schedule)
	assert.Equal(t, "/opt/cleanup.sh", jobs[0].Command)
	assert.False(t, jobs[0].Enabled)
	assert.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), jobs[0].CreatedAt)
}
