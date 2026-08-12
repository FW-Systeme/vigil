package update

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type Logger struct {
	mu  sync.Mutex
	w   io.Writer
	app string
}

func NewLogger(w io.Writer, app string) *Logger {
	return &Logger{w: w, app: app}
}

type logEntry struct {
	Timestamp string         `json:"ts"`
	Event     string         `json:"event"`
	App       string         `json:"app"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func (l *Logger) Log(event string, fields map[string]any) {
	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     event,
		App:       l.app,
		Fields:    fields,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w != nil {
		_, _ = l.w.Write(data)
	}
}
