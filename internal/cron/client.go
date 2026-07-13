package cron

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

var _ Client = (*cronClient)(nil)

type cronClient struct {
	execCommand func(name string, args ...string) *exec.Cmd
}

func NewClient() Client {
	return &cronClient{
		execCommand: exec.Command,
	}
}

// markerLine returns the marker comment for a given job name.
func markerLine(name string) string {
	return fmt.Sprintf("# vigil-cron: %s", name)
}

// parseEntryLine splits a cron entry line into schedule (5 fields) and command.
func parseEntryLine(line string) (schedule, command string) {
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return "", ""
	}
	schedule = strings.Join(parts[:5], " ")
	command = strings.Join(parts[5:], " ")
	return
}

// readLines reads the current system crontab as a slice of lines.
// Returns an empty slice if no crontab exists.
func (c *cronClient) readLines() ([]string, error) {
	cmd := c.execCommand("crontab", "-l")
	out, err := cmd.Output()
	if err != nil {
		// crontab -l exits non-zero when no crontab exists
		return []string{}, nil
	}
	content := strings.TrimRight(string(out), "\n")
	if content == "" {
		return []string{}, nil
	}
	return strings.Split(content, "\n"), nil
}

// writeLines replaces the system crontab with the given lines.
func (c *cronClient) writeLines(lines []string) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	cmd := c.execCommand("crontab", "-")
	cmd.Stdin = &buf
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("writing crontab: %w (output: %s)", err, string(out))
	}
	return nil
}

// findEntry locates a vigil-cron entry by name in the given lines.
// Returns the index of the entry line and whether it is disabled.
func findEntry(lines []string, name string) (entryIdx int, disabled bool, found bool) {
	marker := markerLine(name)
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != marker {
			continue
		}
		if i+1 >= len(lines) {
			return 0, false, false
		}
		entryLine := strings.TrimSpace(lines[i+1])
		return i + 1, strings.HasPrefix(entryLine, "#DISABLED# "), true
	}
	return 0, false, false
}

// extractJobs parses all vigil-cron entries from the given lines.
func extractJobs(lines []string) []Job {
	var jobs []Job
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "# vigil-cron:") {
			continue
		}

		name := strings.TrimSpace(strings.TrimPrefix(line, "# vigil-cron:"))
		if name == "" {
			continue
		}

		// Entry line is the next line
		i++
		if i >= len(lines) {
			break
		}

		entryLine := strings.TrimSpace(lines[i])
		disabled := strings.HasPrefix(entryLine, "#DISABLED# ")

		cleanLine := entryLine
		if disabled {
			cleanLine = strings.TrimPrefix(cleanLine, "#DISABLED# ")
		}

		schedule, command := parseEntryLine(cleanLine)
		if schedule == "" {
			continue
		}

		jobs = append(jobs, Job{
			Name:     name,
			Schedule: schedule,
			Command:  command,
			Enabled:  !disabled,
		})
	}
	return jobs
}

func (c *cronClient) Install(job Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	lines, err := c.readLines()
	if err != nil {
		return err
	}

	// Check for duplicate name
	for _, j := range extractJobs(lines) {
		if j.Name == job.Name {
			return ErrAlreadyExists
		}
	}

	lines = append(lines, markerLine(job.Name), job.Schedule+" "+job.Command)

	return c.writeLines(lines)
}

func (c *cronClient) Remove(name string) error {
	lines, err := c.readLines()
	if err != nil {
		return err
	}

	entryIdx, _, found := findEntry(lines, name)
	if !found {
		return ErrNotFound
	}

	newLines := make([]string, 0, len(lines)-2)
	newLines = append(newLines, lines[:entryIdx-1]...)
	newLines = append(newLines, lines[entryIdx+1:]...)

	return c.writeLines(newLines)
}

func (c *cronClient) Enable(name string) error {
	lines, err := c.readLines()
	if err != nil {
		return err
	}

	entryIdx, disabled, found := findEntry(lines, name)
	if !found {
		return ErrNotFound
	}

	if !disabled {
		return nil
	}

	lines[entryIdx] = strings.TrimPrefix(lines[entryIdx], "#DISABLED# ")

	return c.writeLines(lines)
}

func (c *cronClient) Disable(name string) error {
	lines, err := c.readLines()
	if err != nil {
		return err
	}

	entryIdx, disabled, found := findEntry(lines, name)
	if !found {
		return ErrNotFound
	}

	if disabled {
		return nil
	}

	lines[entryIdx] = "#DISABLED# " + lines[entryIdx]

	return c.writeLines(lines)
}

func (c *cronClient) List() ([]Job, error) {
	lines, err := c.readLines()
	if err != nil {
		return nil, err
	}

	return extractJobs(lines), nil
}

func (c *cronClient) Get(name string) (Job, error) {
	lines, err := c.readLines()
	if err != nil {
		return Job{}, err
	}

	for _, job := range extractJobs(lines) {
		if job.Name == name {
			return job, nil
		}
	}

	return Job{}, ErrNotFound
}

func (c *cronClient) IsCronRunning() (bool, error) {
	// Try systemctl
	cmd := c.execCommand("systemctl", "is-active", "cron")
	if out, err := cmd.Output(); err == nil {
		if strings.TrimSpace(string(out)) == "active" {
			return true, nil
		}
	}

	// Try service command
	cmd = c.execCommand("service", "cron", "status")
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Try pgrep
	cmd = c.execCommand("pgrep", "-x", "crond")
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	return false, nil
}
