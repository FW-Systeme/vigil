//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitEcosystem(t *testing.T) {
	path := "/tmp/vigil-e2e-ecosystem-init.json"
	os.Remove(path)
	t.Cleanup(func() { os.Remove(path) })

	res := RunVigil("init", "-o", path)
	RequireSuccess(t, res, "vigil init")
	assert.True(t, FileExists(path), "ecosystem file should exist")

	data, err := ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, data, "name", "should contain name field")
	assert.Contains(t, data, "my-app", "should contain default name")
	assert.Contains(t, data, "port", "should contain port field")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(data), &raw))
	_, ok := raw["name"]
	assert.True(t, ok, "should have name key")
}

func TestInitCronfile(t *testing.T) {
	path := "/tmp/vigil-e2e-cronfile-init.json"
	os.Remove(path)
	t.Cleanup(func() { os.Remove(path) })

	res := RunVigil("cron", "init", "-o", path)
	RequireSuccess(t, res, "vigil cron init")
	assert.True(t, FileExists(path), "cronfile should exist")

	data, err := ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, data, "crons", "should contain crons array")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(data), &raw))
	_, ok := raw["crons"]
	assert.True(t, ok, "should have crons key")
}
