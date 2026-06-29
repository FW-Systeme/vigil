package process

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
