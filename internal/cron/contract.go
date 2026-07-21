package cron

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrNotFound       = errors.New("cron job not found")
	ErrAlreadyExists  = errors.New("cron job already exists")
	ErrCronNotRunning = errors.New("cron daemon is not running")
)

type Job struct {
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Command   string    `json:"command"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

func (j Job) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("name is required")
	}
	if j.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if j.Command == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}

type File struct {
	Crons []Job `json:"crons"`
}

func ParseCronFile(r io.Reader) ([]Job, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading cron file: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if cronsRaw, ok := raw["crons"]; ok {
		var crons []Job
		if err := json.Unmarshal(cronsRaw, &crons); err != nil {
			return nil, fmt.Errorf("invalid crons array: %w", err)
		}
		if len(crons) == 0 {
			return nil, fmt.Errorf("crons array is empty")
		}
		return crons, nil
	}

	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("invalid cron job config: %w", err)
	}
	if j.Name == "" {
		return nil, fmt.Errorf("cron file must contain either a 'crons' array or a valid job object")
	}
	return []Job{j}, nil
}

type Store interface {
	Save(job Job) error
	Get(name string) (Job, error)
	List() ([]Job, error)
	Delete(name string) error
}

type Client interface {
	Install(job Job) error
	Remove(name string) error
	Enable(name string) error
	Disable(name string) error
	List() ([]Job, error)
	Get(name string) (Job, error)
	IsCronRunning() (bool, error)
}
