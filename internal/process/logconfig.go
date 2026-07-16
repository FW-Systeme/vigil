package process

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LogConfig holds persistent log-save configuration for a managed app.
type LogConfig struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	LogPath string `json:"log_path"`
	MaxSize string `json:"max_size"`
	Rotate  int    `json:"rotate"`
}

// LogStore persists LogConfig entries.
type LogStore interface {
	Load(name string) (LogConfig, error)
	Save(cfg LogConfig) error
	Delete(name string) error
	List() ([]LogConfig, error)
}

var _ LogStore = (*jsonFileLogStore)(nil)

type jsonFileLogStore struct {
	storageDir string
}

func NewLogStore() (LogStore, error) {
	baseDir := os.Getenv("VIRGIL_HOME")
	if baseDir == "" {
		if os.Geteuid() == 0 {
			baseDir = "/etc/vigil"
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			baseDir = filepath.Join(home, ".config", "vigil")
		}
	}
	storageDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}
	return &jsonFileLogStore{storageDir: storageDir}, nil
}

func (s *jsonFileLogStore) path(name string) string {
	return filepath.Join(s.storageDir, name+".json")
}

func (s *jsonFileLogStore) Load(name string) (LogConfig, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		return LogConfig{}, err
	}
	var cfg LogConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return LogConfig{}, err
	}
	return cfg, nil
}

func (s *jsonFileLogStore) Save(cfg LogConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.CreateTemp(s.storageDir, "*.tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpName)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path(cfg.Name)); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *jsonFileLogStore) Delete(name string) error {
	err := os.Remove(s.path(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *jsonFileLogStore) List() ([]LogConfig, error) {
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogConfig{}, nil
		}
		return nil, err
	}

	var configs []LogConfig
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.storageDir, entry.Name()))
		if err != nil {
			continue
		}
		var cfg LogConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}
