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

// ParseLevel converts an arbitrary string into a Level. It performs an
// allocation-free, ASCII case-insensitive comparison (no strings.ToLower /
// TrimSpace over a freshly allocated string).
func ParseLevel(s string) Level {
	s = strings.TrimSpace(s)
	if s == "" {
		return LevelUnknown
	}
	// Dispatch on the (folded) first byte to avoid comparing against every
	// keyword; then confirm with an allocation-free case-insensitive compare.
	c := s[0]
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	switch c {
	case 't':
		if equalFoldASCII(s, "trace") {
			return LevelTrace
		}
	case 'd':
		if equalFoldASCII(s, "debug") || equalFoldASCII(s, "dbg") {
			return LevelDebug
		}
	case 'i':
		if equalFoldASCII(s, "info") || equalFoldASCII(s, "informational") {
			return LevelInfo
		}
	case 'n':
		if equalFoldASCII(s, "notice") {
			return LevelInfo
		}
	case 'w':
		if equalFoldASCII(s, "warn") || equalFoldASCII(s, "warning") {
			return LevelWarn
		}
	case 'e':
		if equalFoldASCII(s, "error") || equalFoldASCII(s, "err") {
			return LevelError
		}
		if equalFoldASCII(s, "emerg") || equalFoldASCII(s, "emergency") {
			return LevelFatal
		}
	case 's':
		if equalFoldASCII(s, "severe") {
			return LevelError
		}
	case 'f':
		if equalFoldASCII(s, "fatal") {
			return LevelFatal
		}
	case 'p':
		if equalFoldASCII(s, "panic") {
			return LevelFatal
		}
	case 'c':
		if equalFoldASCII(s, "critical") || equalFoldASCII(s, "crit") {
			return LevelFatal
		}
	case 'a':
		if equalFoldASCII(s, "alert") {
			return LevelFatal
		}
	}
	return LevelUnknown
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

// Layout groups, kept in the same relative order as the original flat list so
// that classification never changes which layout wins for a given input.
var (
	// "2006-01-02T15:04:05..." (ISO-8601 with a 'T' date/time separator).
	layoutsT = []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05",
	}
	// "2006-01-02 15:04:05" with an optional fractional part (Java/Python).
	layoutsSpace = []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05,000",
		"2006-01-02 15:04:05.000",
	}
	// Slash-separated dates: Nginx error ("2006/01/02 ...") and
	// Apache/Nginx access ("02/Jan/2006:...").
	layoutsSlash = []string{
		"2006/01/02 15:04:05",
		"02/Jan/2006:15:04:05 -0700",
	}
	// syslog RFC3164 ("Jan _2 15:04:05").
	layoutSyslog = "Jan _2 15:04:05"
)

// parseTime tries only the layouts compatible with the shape of s, classified
// by a few cheap structural markers. The relative order of candidate layouts
// matches the original sequential list, so results are identical while far
// fewer time.Parse attempts (and their allocations) are made.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	var candidates []string
	switch {
	case len(s) >= 11 && s[10] == 'T':
		candidates = layoutsT
	case len(s) >= 11 && s[4] == '-' && s[10] == ' ':
		candidates = layoutsSpace
	case strings.IndexByte(s, '/') >= 0:
		candidates = layoutsSlash
	case isUpperLetter(s[0]):
		if t, err := time.Parse(layoutSyslog, s); err == nil {
			return t, true
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
	for _, l := range candidates {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
