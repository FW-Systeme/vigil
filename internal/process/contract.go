package process

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Type string

const (
	TypeApp    Type = "app"
	TypeNode   Type = "node"
	TypeStatic Type = "static"
)

type Process struct {
	Name            string    `json:"name"`
	Type            Type      `json:"type"`
	Port            int       `json:"port"`
	Entry           string    `json:"entry,omitempty"`
	BuildDir        string    `json:"build_dir,omitempty"`
	EnvFile         string    `json:"env_file,omitempty"`
	WorkingDir      string    `json:"working_dir,omitempty"`
	NginxDomain     string    `json:"nginx_domain,omitempty"`
	NginxPath       string    `json:"nginx_path,omitempty"`
	NginxConfig     string    `json:"nginx_config,omitempty"`
	Command         string    `json:"command,omitempty"`
	BuildCmd        string    `json:"build_cmd,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	Enabled         bool      `json:"enabled"`
	SmokeTestScript string    `json:"smoke_test_script,omitempty"`
	BundledDeps     bool      `json:"bundled_deps,omitempty"`
	InstallCmd      string    `json:"install_cmd,omitempty"`
	KillMode        string    `json:"kill_mode,omitempty"`
}

func (p Process) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if p.Type != TypeApp && p.Type != TypeNode && p.Type != TypeStatic {
		return fmt.Errorf("type must be %q, %q, or %q", TypeApp, TypeNode, TypeStatic)
	}
	if p.Port <= 0 {
		return fmt.Errorf("port must be a positive integer")
	}
	if p.IsBackend() && p.Entry == "" && p.Command == "" {
		return fmt.Errorf("entry or command is required for app/node type")
	}
	if p.Type == TypeStatic && p.BuildDir == "" {
		return fmt.Errorf("build_dir is required for static apps")
	}
	if p.SmokeTestScript == "" {
		return fmt.Errorf("smoke_test_script is required")
	}
	if !p.BundledDeps && p.InstallCmd == "" && p.IsBackend() {
		return fmt.Errorf("install_cmd is required when bundled_deps is false")
	}
	validKillModes := map[string]bool{
		"":              true,
		"process":       true,
		"control-group": true,
		"mixed":         true,
		"none":          true,
	}
	if !validKillModes[p.KillMode] {
		return fmt.Errorf("kill_mode must be one of: process, control-group, mixed, none (or empty for default)")
	}
	return nil
}

// IsBackend returns true if the process is a backend type (app or node).
func (p Process) IsBackend() bool {
	return p.Type == TypeApp || p.Type == TypeNode
}

type EcosystemFile struct {
	Apps []Process `json:"apps"`
}

func ParseEcosystemFile(r io.Reader) ([]Process, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading ecosystem file: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if appsRaw, ok := raw["apps"]; ok {
		var apps []Process
		if err := json.Unmarshal(appsRaw, &apps); err != nil {
			return nil, fmt.Errorf("invalid apps array: %w", err)
		}
		if len(apps) == 0 {
			return nil, fmt.Errorf("apps array is empty")
		}
		return apps, nil
	}

	var p Process
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid process config: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("ecosystem file must contain either an 'apps' array or a valid process object")
	}
	return []Process{p}, nil
}

type Store interface {
	Load(name string) (Process, error)
	Save(p Process) error
	Delete(name string) error
	List() ([]Process, error)
	AppPath(name string) (string, error)
}
