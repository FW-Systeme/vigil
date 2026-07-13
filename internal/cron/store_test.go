package cron

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStore struct {
	Store
	dir string
}

func newTestStore(t *testing.T) testStore {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	s, err := NewStore()
	require.NoError(t, err)
	return testStore{Store: s, dir: filepath.Join(dir, ".config", "vigil", "cron")}
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
}

func TestStore_SaveAndGet(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	job := Job{
		Name:      "daily-backup",
		Schedule:  "0 3 * * *",
		Command:   "/usr/bin/backup",
		Enabled:   true,
		CreatedAt: now,
	}

	err := store.Save(job)
	require.NoError(t, err)

	got, err := store.Get("daily-backup")
	require.NoError(t, err)
	assert.Equal(t, job, got)
}

func TestStore_SaveAndList(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	jobs := []Job{
		{Name: "alpha", Schedule: "0 1 * * *", Command: "cmd_a", Enabled: true, CreatedAt: now},
		{Name: "beta", Schedule: "0 2 * * *", Command: "cmd_b", Enabled: false, CreatedAt: now.Add(time.Hour)},
		{Name: "gamma", Schedule: "0 3 * * *", Command: "cmd_c", Enabled: true, CreatedAt: now.Add(2 * time.Hour)},
	}

	for _, j := range jobs {
		err := store.Save(j)
		require.NoError(t, err)
	}

	loaded, err := store.List()
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	byName := make(map[string]Job, len(loaded))
	for _, j := range loaded {
		byName[j.Name] = j
	}

	for _, expected := range jobs {
		got, ok := byName[expected.Name]
		if assert.True(t, ok, "job %q should be in List result", expected.Name) {
			assert.Equal(t, expected, got)
		}
	}
}

func TestStore_Delete_Existing(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	job := Job{Name: "delete-me", Schedule: "0 4 * * *", Command: "/bin/true", Enabled: true, CreatedAt: now}
	err := store.Save(job)
	require.NoError(t, err)

	_, err = store.Get("delete-me")
	require.NoError(t, err)

	err = store.Delete("delete-me")
	require.NoError(t, err)

	_, err = store.Get("delete-me")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Delete_NonExistent(t *testing.T) {
	store := newTestStore(t)

	err := store.Delete("never-created")
	require.NoError(t, err)
}

func TestStore_List_Empty(t *testing.T) {
	store := newTestStore(t)

	jobs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestStore_OverwriteExisting(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	original := Job{Name: "updatable", Schedule: "0 1 * * *", Command: "v1", Enabled: true, CreatedAt: now}
	err := store.Save(original)
	require.NoError(t, err)

	updated := Job{Name: "updatable", Schedule: "0 2 * * *", Command: "v2", Enabled: false, CreatedAt: now.Add(time.Hour)}
	err = store.Save(updated)
	require.NoError(t, err)

	got, err := store.Get("updatable")
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

func TestStore_Get_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get("does-not-exist")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_SaveMultipleAndDeleteOne(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	for _, j := range []Job{
		{Name: "a", Schedule: "0 1 * * *", Command: "c1", Enabled: true, CreatedAt: now},
		{Name: "b", Schedule: "0 2 * * *", Command: "c2", Enabled: false, CreatedAt: now},
		{Name: "c", Schedule: "0 3 * * *", Command: "c3", Enabled: true, CreatedAt: now},
	} {
		require.NoError(t, store.Save(j))
	}

	require.NoError(t, store.Delete("b"))

	list, err := store.List()
	require.NoError(t, err)
	require.Len(t, list, 2)

	_, err = store.Get("b")
	require.Error(t, err)

	_, err = store.Get("a")
	require.NoError(t, err)
	_, err = store.Get("c")
	require.NoError(t, err)

	names := make(map[string]bool, 2)
	for _, j := range list {
		names[j.Name] = true
	}
	assert.True(t, names["a"])
	assert.True(t, names["c"])
	assert.False(t, names["b"])
}

func TestStore_Persistence_SameDirectory(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("HOME", dir)
	store1, err := NewStore()
	require.NoError(t, err)

	job := Job{Name: "persist-test", Schedule: "0 5 * * *", Command: "/bin/persist", Enabled: true, CreatedAt: fixedTime()}
	err = store1.Save(job)
	require.NoError(t, err)

	store2, err := NewStore()
	require.NoError(t, err)

	got, err := store2.Get("persist-test")
	require.NoError(t, err)
	assert.Equal(t, job, got)
}

func TestStore_AutoCreateDirectory(t *testing.T) {
	store := newTestStore(t)

	info, err := os.Stat(store.dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
	}
}

func TestStore_List_IgnoresNonJSONFiles(t *testing.T) {
	store := newTestStore(t)

	nonJSON := filepath.Join(store.dir, "ignored.txt")
	err := os.WriteFile(nonJSON, []byte("garbage"), 0600)
	require.NoError(t, err)

	jobs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestStore_List_MalformedJSON(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	good := Job{Name: "good-job", Schedule: "0 6 * * *", Command: "/bin/good", Enabled: true, CreatedAt: now}
	err := store.Save(good)
	require.NoError(t, err)

	badPath := filepath.Join(store.dir, "bad.json")
	err = os.WriteFile(badPath, []byte("{invalid json"), 0600)
	require.NoError(t, err)

	jobs, err := store.List()
	require.NoError(t, err)

	found := false
	for _, j := range jobs {
		if j.Name == "good-job" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestStore_List_IgnoresSubdirectories(t *testing.T) {
	store := newTestStore(t)

	subDir := filepath.Join(store.dir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, "ignored.json"), []byte("{}"), 0600)
	require.NoError(t, err)

	jobs, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestStore_Save_Atomicity(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	job := Job{Name: "atomic-test", Schedule: "0 7 * * *", Command: "/bin/atomic", Enabled: true, CreatedAt: now}
	err := store.Save(job)
	require.NoError(t, err)

	matches, err := filepath.Glob(filepath.Join(store.dir, "atomic-test*.json"))
	require.NoError(t, err)
	assert.Len(t, matches, 1)

	tmpFiles, err := filepath.Glob(filepath.Join(store.dir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, tmpFiles)
}

func TestStore_List_StorageDirIsFile(t *testing.T) {
	store := newTestStore(t)

	err := os.RemoveAll(store.dir)
	require.NoError(t, err)

	err = os.WriteFile(store.dir, []byte("not-a-dir"), 0600)
	require.NoError(t, err)

	_, err = store.List()
	require.Error(t, err)
}

func TestStore_Load_CorruptFile(t *testing.T) {
	store := newTestStore(t)

	badPath := filepath.Join(store.dir, "corrupt.json")
	err := os.WriteFile(badPath, []byte("{broken"), 0600)
	require.NoError(t, err)

	_, err = store.Get("corrupt")
	require.Error(t, err)
}

func TestStore_SaveAndGet_MinimalFields(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	job := Job{Name: "minimal", Schedule: "0 8 * * *", Command: "/bin/minimal", Enabled: false, CreatedAt: now}
	err := store.Save(job)
	require.NoError(t, err)

	got, err := store.Get("minimal")
	require.NoError(t, err)
	assert.Equal(t, job, got)
}

func TestStore_SaveAndGet_SpecialName(t *testing.T) {
	store := newTestStore(t)
	now := fixedTime()

	job := Job{Name: "app-with.dots_and-dashes", Schedule: "0 9 * * *", Command: "/bin/special", Enabled: true, CreatedAt: now}
	err := store.Save(job)
	require.NoError(t, err)

	got, err := store.Get("app-with.dots_and-dashes")
	require.NoError(t, err)
	assert.Equal(t, job, got)
}

func TestStore_NewStore_MkdirAllError(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, ".config", "vigil")
	err := os.MkdirAll(filepath.Dir(configPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(configPath, []byte("not a directory"), 0600)
	require.NoError(t, err)

	t.Setenv("HOME", dir)
	_, err = NewStore()
	require.Error(t, err)
}

func TestStore_Save_PermissionDenied(t *testing.T) {
	store := newTestStore(t)

	err := os.Chmod(store.dir, 0555)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chmod(store.dir, 0755) })

	job := Job{Name: "no-perm", Schedule: "0 10 * * *", Command: "/bin/noperm", Enabled: true, CreatedAt: fixedTime()}
	err = store.Save(job)
	require.Error(t, err)
}
