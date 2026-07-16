//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCronAdd(t *testing.T) {
	name := "e2e-cron-add"
	cleanupDeferCron(t, name)

	cronCmd := fmt.Sprintf("echo 'vigil-e2e-%s' >> /tmp/vigil-e2e-cron-add.log", name)
	res := RunVigil("cron", "add", name,
		"--schedule=*/5 * * * *",
		fmt.Sprintf("--command=%s", cronCmd),
	)
	RequireSuccess(t, res, "vigil cron add")

	assert.True(t, CrontabContains(name), "crontab should contain job name")
	assert.True(t, FileExists(CronStoreFile(name)), "store file should exist")
}

func TestCronRemove(t *testing.T) {
	name := "e2e-cron-remove"

	addCronJob(t, name, "/tmp/vigil-e2e-cron-remove.log")

	res := RunVigil("cron", "remove", name)
	RequireSuccess(t, res, "vigil cron remove")

	assert.False(t, CrontabContains(name), "crontab should not contain job")
	assert.False(t, FileExists(CronStoreFile(name)), "store file should be deleted")
}

func TestCronEnable(t *testing.T) {
	name := "e2e-cron-enable"
	cleanupDeferCron(t, name)

	addCronJob(t, name, "/tmp/vigil-e2e-cron-enable.log")

	res := RunVigil("cron", "disable", name)
	RequireSuccess(t, res, "vigil cron disable")

	res = RunVigil("cron", "enable", name)
	RequireSuccess(t, res, "vigil cron enable")
}

func TestCronDisable(t *testing.T) {
	name := "e2e-cron-disable"
	cleanupDeferCron(t, name)

	addCronJob(t, name, "/tmp/vigil-e2e-cron-disable.log")

	res := RunVigil("cron", "disable", name)
	RequireSuccess(t, res, "vigil cron disable")
}

func TestCronStatus(t *testing.T) {
	name := "e2e-cron-status"
	cleanupDeferCron(t, name)

	addCronJob(t, name, "/tmp/vigil-e2e-cron-status.log")

	res := RunVigil("cron", "status", name)
	RequireSuccess(t, res, "vigil cron status")
	assert.Contains(t, res.Stdout, "active", "status should show active")
}

func TestCronInvalidSchedule(t *testing.T) {
	name := "e2e-cron-invalid"
	cleanupDeferCron(t, name)

	res := RunVigil("cron", "add", name,
		"--schedule=invalid",
		"--command=true",
	)
	RequireError(t, res, "add cron with invalid schedule")
}

func addCronJob(t *testing.T, name, logFile string) {
	t.Helper()
	cronCmd := fmt.Sprintf("echo 'vigil-e2e-%s' >> %s", name, logFile)
	res := RunVigil("cron", "add", name,
		"--schedule=*/5 * * * *",
		fmt.Sprintf("--command=%s", cronCmd),
	)
	RequireSuccess(t, res, "adding cron job "+name)
}

func cleanupDeferCron(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		RunVigil("cron", "remove", name)
	})
}
