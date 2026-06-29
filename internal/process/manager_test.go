package process

import (
	"context"
	"fmt"
	"testing"

	"github.com/chris576/vigil/internal/nginx"
	"github.com/chris576/vigil/internal/systemd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSystemd struct {
	systemd.Client
	startCalled   bool
	stopCalled    bool
	restartCalled bool
	createCalled  bool
	removeCalled  bool
	enableCalled  bool
	disableCalled bool
	reloadCalled  bool
	statusActive  string
	statusSub     string
	err           error
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
	m.restartCalled = true
	return m.err
}
func (m *mockSystemd) EnableUnit(ctx context.Context, name string) error {
	m.enableCalled = true
	return m.err
}
func (m *mockSystemd) DisableUnit(ctx context.Context, name string) error {
	m.disableCalled = true
	return m.err
}
func (m *mockSystemd) UnitStatus(ctx context.Context, name string) (string, string, error) {
	return m.statusActive, m.statusSub, m.err
}
func (m *mockSystemd) CreateUnitFile(name string, content []byte) error {
	m.createCalled = true
	return m.err
}
func (m *mockSystemd) RemoveUnitFile(name string) error {
	m.removeCalled = true
	return m.err
}
func (m *mockSystemd) Reload(ctx context.Context) error {
	m.reloadCalled = true
	return m.err
}
func (m *mockSystemd) Close() error { return nil }

type mockNginx struct {
	nginx.Client
	enableCalled   bool
	disableCalled  bool
	removeCalled   bool
	siteEnabledVal bool
	err            error
}

func (m *mockNginx) EnableSite(name string, port int, domain, root string) error {
	m.enableCalled = true
	return m.err
}
func (m *mockNginx) DisableSite(name string) error {
	m.disableCalled = true
	return m.err
}
func (m *mockNginx) RemoveSiteConfig(name string) error {
	m.removeCalled = true
	return m.err
}
func (m *mockNginx) SiteEnabled(name string) (bool, error) {
	return m.siteEnabledVal, m.err
}
func (m *mockNginx) Reload(ctx context.Context) error {
	return m.err
}
func (m *mockNginx) Close() error { return nil }

type mockStore struct {
	Store
	processes map[string]Process
	err       error
}

func (m *mockStore) Load(name string) (Process, error) {
	p, ok := m.processes[name]
	if !ok {
		return Process{}, fmt.Errorf("not found")
	}
	return p, nil
}
func (m *mockStore) Save(p Process) error { return m.err }
func (m *mockStore) Delete(name string) error {
	delete(m.processes, name)
	return m.err
}
func (m *mockStore) List() ([]Process, error) {
	var list []Process
	for _, p := range m.processes {
		list = append(list, p)
	}
	return list, m.err
}

func TestManager_AddProcess_Node_Success(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, ng)

	p := Process{Name: "test-app", Type: TypeNode, Port: 3000, Entry: "app.js", WorkingDir: "/app"}
	err := m.AddProcess(context.Background(), p, false)
	require.NoError(t, err)
	assert.True(t, sd.createCalled, "CreateUnitFile should be called")
	assert.True(t, sd.enableCalled, "EnableUnit should be called")
	assert.True(t, sd.reloadCalled, "Reload should be called")
}

func TestManager_AddProcess_Static_Success(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, ng)

	p := Process{Name: "test-site", Type: TypeStatic, Port: 8080, BuildDir: "./dist", NginxDomain: "example.com", NginxPath: "/var/www"}
	err := m.AddProcess(context.Background(), p, false)
	require.NoError(t, err)
	assert.True(t, ng.enableCalled, "EnableSite should be called")
}

func TestManager_AddProcess_Duplicate_NoForce(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"existing": {Name: "existing"}}}
	m := New(store, sd, ng)

	p := Process{Name: "existing", Type: TypeNode, Port: 3000, Entry: "app.js"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestManager_AddProcess_Duplicate_Force(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"existing": {Name: "existing"}}}
	m := New(store, sd, ng)

	p := Process{Name: "existing", Type: TypeNode, Port: 3000, Entry: "app.js", WorkingDir: "/app"}
	err := m.AddProcess(context.Background(), p, true)
	require.NoError(t, err)
}

func TestManager_AddProcess_InvalidType(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, ng)

	p := Process{Name: "bad", Type: "invalid", Port: 3000}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
}

func TestManager_AddProcess_NilSystemd(t *testing.T) {
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, nil, ng)

	p := Process{Name: "test", Type: TypeNode, Port: 3000, Entry: "app.js"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "systemd client not available")
}

func TestManager_AddProcess_NilNginx(t *testing.T) {
	sd := &mockSystemd{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, nil)

	p := Process{Name: "test", Type: TypeStatic, Port: 8080, BuildDir: "./dist"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nginx client not available")
}

func TestManager_RemoveProcess_Node_Success(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"svc": {Name: "svc", Type: TypeNode}}}
	m := New(store, sd, ng)

	err := m.RemoveProcess(context.Background(), "svc")
	require.NoError(t, err)
	assert.True(t, sd.stopCalled)
	assert.True(t, sd.disableCalled)
	assert.True(t, sd.removeCalled)
	assert.True(t, sd.reloadCalled)
	_, exists := store.processes["svc"]
	assert.False(t, exists)
}

func TestManager_RemoveProcess_Static_Success(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic}}}
	m := New(store, sd, ng)

	err := m.RemoveProcess(context.Background(), "site")
	require.NoError(t, err)
	assert.True(t, ng.disableCalled)
	assert.True(t, ng.removeCalled)
}

func TestManager_RemoveProcess_NotFound(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, ng)

	err := m.RemoveProcess(context.Background(), "ghost")
	require.Error(t, err)
}

func TestManager_ListProcesses(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{
		"a": {Name: "a", Type: TypeNode},
		"b": {Name: "b", Type: TypeStatic},
	}}
	m := New(store, sd, ng)

	list, err := m.ListProcesses(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestManager_StartProcess_Node(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"svc": {Name: "svc", Type: TypeNode}}}
	m := New(store, sd, ng)

	err := m.StartProcess(context.Background(), "svc")
	require.NoError(t, err)
	assert.True(t, sd.startCalled)
}

func TestManager_StartProcess_Static(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic, Port: 8080}}}
	m := New(store, sd, ng)

	err := m.StartProcess(context.Background(), "site")
	require.NoError(t, err)
	assert.True(t, ng.enableCalled)
}

func TestManager_StopProcess_Node(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"svc": {Name: "svc", Type: TypeNode}}}
	m := New(store, sd, ng)

	err := m.StopProcess(context.Background(), "svc")
	require.NoError(t, err)
	assert.True(t, sd.stopCalled)
}

func TestManager_StopProcess_Static(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic}}}
	m := New(store, sd, ng)

	err := m.StopProcess(context.Background(), "site")
	require.NoError(t, err)
	assert.True(t, ng.disableCalled)
}

func TestManager_RestartProcess_Node(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"svc": {Name: "svc", Type: TypeNode}}}
	m := New(store, sd, ng)

	err := m.RestartProcess(context.Background(), "svc")
	require.NoError(t, err)
	assert.True(t, sd.restartCalled)
}

func TestManager_RestartProcess_Static(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic, Port: 8080}}}
	m := New(store, sd, ng)

	err := m.RestartProcess(context.Background(), "site")
	require.NoError(t, err)
	assert.True(t, ng.disableCalled)
	assert.True(t, ng.enableCalled)
}

func TestManager_Status_Node(t *testing.T) {
	sd := &mockSystemd{statusActive: "active", statusSub: "running"}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"svc": {Name: "svc", Type: TypeNode}}}
	m := New(store, sd, ng)

	active, sub, err := m.Status(context.Background(), "svc")
	require.NoError(t, err)
	assert.Equal(t, "active", active)
	assert.Equal(t, "running", sub)
}

func TestManager_Status_Static_Enabled(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{siteEnabledVal: true}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic}}}
	m := New(store, sd, ng)

	active, sub, err := m.Status(context.Background(), "site")
	require.NoError(t, err)
	assert.Equal(t, "active", active)
	assert.Equal(t, "enabled", sub)
}

func TestManager_Status_Static_Disabled(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{siteEnabledVal: false}
	store := &mockStore{processes: map[string]Process{"site": {Name: "site", Type: TypeStatic}}}
	m := New(store, sd, ng)

	active, sub, err := m.Status(context.Background(), "site")
	require.NoError(t, err)
	assert.Equal(t, "inactive", active)
	assert.Equal(t, "disabled", sub)
}

func TestManager_Status_UnknownType(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: "unknown"}}}
	m := New(store, nil, nil)
	_, _, err := m.Status(context.Background(), "x")
	require.Error(t, err)
}

func TestUnitContent_WithoutEnvFile(t *testing.T) {
	p := Process{Name: "my-app", Type: TypeNode, WorkingDir: "/app", Entry: "server.js"}
	content := string(unitContent(p))
	assert.Contains(t, content, "Description=Vigil: my-app")
	assert.Contains(t, content, "WorkingDirectory=/app")
	assert.Contains(t, content, "ExecStart=/usr/bin/node server.js")
	assert.NotContains(t, content, "EnvironmentFile")
}

func TestUnitContent_WithEnvFile(t *testing.T) {
	p := Process{Name: "my-app", Type: TypeNode, WorkingDir: "/app", Entry: "server.js", EnvFile: "/app/.env"}
	content := string(unitContent(p))
	assert.Contains(t, content, "EnvironmentFile=/app/.env")
}

func TestManager_StartProcess_UnknownType(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: "unknown"}}}
	m := New(store, nil, nil)
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_UnknownType(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: "unknown"}}}
	m := New(store, nil, nil)
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RestartProcess_UnknownType(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: "unknown"}}}
	m := New(store, nil, nil)
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StartProcess_NodeNilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "systemd client not available")
}

func TestManager_StartProcess_StaticNilNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic, Port: 8080}}}
	m := New(store, &mockSystemd{}, nil)
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nginx client not available")
}

func TestManager_ListProcesses_StoreError(t *testing.T) {
	store := &mockStore{processes: map[string]Process{}, err: fmt.Errorf("store error")}
	m := New(store, nil, nil)
	_, err := m.ListProcesses(context.Background())
	require.Error(t, err)
}

func TestManager_Status_NodeNilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	_, _, err := m.Status(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "systemd client not available")
}

func TestManager_Status_StaticNilNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, nil)
	_, _, err := m.Status(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nginx client not available")
}

func TestManager_AddProcess_StoreSaveError(t *testing.T) {
	store := &mockStore{processes: map[string]Process{}, err: fmt.Errorf("save failed")}
	m := New(store, &mockSystemd{}, &mockNginx{})
	p := Process{Name: "test", Type: TypeNode, Port: 3000, Entry: "app.js"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
}

func TestManager_AddProcess_SystemdCreateError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("create failed")}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, &mockNginx{})
	p := Process{Name: "test", Type: TypeNode, Port: 3000, Entry: "app.js", WorkingDir: "/app"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
}

func TestManager_RemoveProcess_SystemdStopError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("stop failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, sd, &mockNginx{})
	err := m.RemoveProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RemoveProcess_StaticNginxError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("disable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.RemoveProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RemoveProcess_NilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.RemoveProcess(context.Background(), "x")
	require.NoError(t, err)
	_, exists := store.processes["x"]
	assert.False(t, exists)
}

func TestManager_RemoveProcess_NilNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, nil)
	err := m.RemoveProcess(context.Background(), "x")
	require.NoError(t, err)
	_, exists := store.processes["x"]
	assert.False(t, exists)
}

func TestManager_RestartProcess_NilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RestartProcess_NilNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic, Port: 8080}}}
	m := New(store, &mockSystemd{}, nil)
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_NilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_NilNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, nil)
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_AddProcess_ValidateError(t *testing.T) {
	m := New(&mockStore{}, &mockSystemd{}, &mockNginx{})
	err := m.AddProcess(context.Background(), Process{Name: ""}, false)
	require.Error(t, err)
}

func TestManager_Status_NodeStatusError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("status error")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, sd, &mockNginx{})
	_, _, err := m.Status(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_Status_StaticSiteEnabledError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("enabled check error")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, ng)
	_, _, err := m.Status(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_AddProcess_NodeNilWorkingDir(t *testing.T) {
	sd := &mockSystemd{}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, &mockNginx{})
	p := Process{Name: "test", Type: TypeNode, Port: 3000, Entry: "app.js"}
	err := m.AddProcess(context.Background(), p, false)
	require.NoError(t, err)
	assert.True(t, sd.createCalled)
}

func TestManager_RemoveProcess_StaticDisableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("disable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.RemoveProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RemoveProcess_NodeNilSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.RemoveProcess(context.Background(), "x")
	require.NoError(t, err)
	_, exists := store.processes["x"]
	assert.False(t, exists)
}

func TestManager_RemoveProcess_NilStoreDeleteError(t *testing.T) {
	sd := &mockSystemd{}
	ng := &mockNginx{}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}, err: fmt.Errorf("delete failed")}
	m := New(store, sd, ng)
	err := m.RemoveProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StartProcess_NilSystemdWithNginx(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, nil, &mockNginx{})
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_NilNginxWithSystemd(t *testing.T) {
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, nil)
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StartProcess_SystemdError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("start failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, sd, &mockNginx{})
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StartProcess_StaticEnableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("enable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic, Port: 8080}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.StartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_SystemdError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("stop failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, sd, &mockNginx{})
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_StopProcess_StaticDisableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("disable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.StopProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RestartProcess_SystemdError(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("restart failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeNode}}}
	m := New(store, sd, &mockNginx{})
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RestartProcess_StaticDisableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("disable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic, Port: 8080}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_RestartProcess_StaticEnableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("enable failed")}
	store := &mockStore{processes: map[string]Process{"x": {Name: "x", Type: TypeStatic, Port: 8080}}}
	m := New(store, &mockSystemd{}, ng)
	err := m.RestartProcess(context.Background(), "x")
	require.Error(t, err)
}

func TestManager_AddProcess_SystemdCreateErrorWithNilNginx(t *testing.T) {
	sd := &mockSystemd{err: fmt.Errorf("create failed")}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, sd, nil)
	p := Process{Name: "test", Type: TypeNode, Port: 3000, Entry: "app.js", WorkingDir: "/app"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
}

func TestManager_AddProcess_NginxEnableError(t *testing.T) {
	ng := &mockNginx{err: fmt.Errorf("enable failed")}
	store := &mockStore{processes: map[string]Process{}}
	m := New(store, nil, ng)
	p := Process{Name: "test", Type: TypeStatic, Port: 8080, BuildDir: "./dist"}
	err := m.AddProcess(context.Background(), p, false)
	require.Error(t, err)
}
