package update

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_SingleWriter(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, nil)

	log.Log("test.event", map[string]any{"key": "value"})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 1)

	var entry logEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &entry))
	assert.Equal(t, "test.event", entry.Event)
	assert.NotEmpty(t, entry.Timestamp)
	assert.Equal(t, "value", entry.Fields["key"])
}

func TestLogger_DualWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	log := NewLogger(&buf1, &buf2)

	log.Log("dual.event", nil)

	assert.Positive(t, buf1.Len())
	assert.Positive(t, buf2.Len())
	assert.Equal(t, buf1.String(), buf2.String())
}

func TestLogger_NilWriters(t *testing.T) {
	log := NewLogger(nil, nil)
	log.Log("silent.event", nil)
}

func TestLogger_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, nil)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			log.Log("concurrent.event", map[string]any{"n": n})
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	lines := buf.String()
	assert.NotEmpty(t, lines)
}

func TestLogger_NoFields(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, nil)

	log.Log("simple.event", nil)

	var entry logEntry
	require.NoError(t, json.Unmarshal(buf.Bytes()[:buf.Len()-1], &entry))
	assert.Equal(t, "simple.event", entry.Event)
	assert.Nil(t, entry.Fields)
}

func TestLogger_JSONValidLines(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, nil)

	log.Log("first", map[string]any{"a": 1})
	log.Log("second", map[string]any{"b": "two"})
	log.Log("third", map[string]any{"c": true})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 3)

	for i, line := range lines {
		var entry logEntry
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "line %d is not valid JSON", i)
		assert.NotEmpty(t, entry.Timestamp)
		assert.NotEmpty(t, entry.Event)
	}
}
