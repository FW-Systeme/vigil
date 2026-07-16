package cron

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var _ Store = (*jsonFileStore)(nil)

type jsonFileStore struct {
	storageDir string
}

func NewStore() (Store, error) {
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

	storageDir := filepath.Join(baseDir, "cron")

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}

	return &jsonFileStore{storageDir: storageDir}, nil
}

func (s *jsonFileStore) path(name string) string {
	return filepath.Join(s.storageDir, name+".json")
}

func (s *jsonFileStore) Save(job Job) error {
	data, err := json.MarshalIndent(job, "", "  ")
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

	if err := os.Rename(tmpName, s.path(job.Name)); err != nil {
		os.Remove(tmpName)
		return err
	}

	return nil
}

func (s *jsonFileStore) Get(name string) (Job, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		return Job{}, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, err
	}

	return job, nil
}

func (s *jsonFileStore) List() ([]Job, error) {
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Job{}, nil
		}
		return nil, err
	}

	var jobs []Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.storageDir, entry.Name()))
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (s *jsonFileStore) Delete(name string) error {
	err := os.Remove(s.path(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
