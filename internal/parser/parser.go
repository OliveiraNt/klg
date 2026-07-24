// Package parser detects and extracts common fields (timestamp, level, message)
// from log lines in several formats.
//
// Parsers are pluggable: each concrete format lives in its own file and
// registers itself via Register in an init function. See Parser for the
// extensibility contract.
package parser

import (
	"strings"
	"time"
)

// Level represents the severity of a log record.
type Level int

const (
	LevelUnknown Level = iota
	LevelTrace
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the canonical uppercase name of the level.
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "LOG"
	}
}

// ParseLevel converts an arbitrary string into a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace
	case "debug", "dbg":
		return LevelDebug
	case "info", "informational", "notice":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error", "err", "severe":
		return LevelError
	case "fatal", "panic", "critical", "crit", "alert", "emerg", "emergency":
		return LevelFatal
	default:
		return LevelUnknown
	}
}

// Entry is a normalized log entry.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  map[string]string
	Raw     string
}

// Parse dispatches the line to the registered parsers in priority order,
// stripping a leading timestamp when present.
func Parse(line string) Entry {
	e := Entry{Raw: line, Fields: map[string]string{}}
	trimmed := strings.TrimSpace(line)

	if ts, rest, ok := splitLeadingTimestamp(trimmed); ok {
		e.Time = ts
		trimmed = rest
	}

	dispatch(trimmed, &e)
	return e
}

func splitLeadingTimestamp(s string) (time.Time, string, bool) {
	idx := strings.IndexByte(s, ' ')
	if idx <= 0 {
		return time.Time{}, s, false
	}
	head := s[:idx]
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, head); err == nil {
			return t, strings.TrimSpace(s[idx+1:]), true
		}
	}
	return time.Time{}, s, false
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05,000", // Java/Python
		"2006-01-02 15:04:05.000",
		"2006/01/02 15:04:05",        // Nginx error
		"02/Jan/2006:15:04:05 -0700", // Apache/Nginx access
		"Jan _2 15:04:05",            // syslog RFC3164
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
