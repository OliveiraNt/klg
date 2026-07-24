// Package parser detects and extracts common fields (timestamp, level, message)
// from log lines in several formats: JSON, logfmt and free-form text.
package parser

import (
	"encoding/json"
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
	case "fatal", "panic", "critical", "crit":
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

// Parse tries to interpret the line as JSON, then logfmt, and finally free-form text.
func Parse(line string) Entry {
	e := Entry{Raw: line, Fields: map[string]string{}}
	trimmed := strings.TrimSpace(line)

	if ts, rest, ok := splitLeadingTimestamp(trimmed); ok {
		e.Time = ts
		trimmed = rest
	}

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		if parseJSON(trimmed, &e) {
			return e
		}
	}
	if looksLikeLogfmt(trimmed) {
		if parseLogfmt(trimmed, &e) {
			return e
		}
	}
	parsePlain(trimmed, &e)
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

func parseJSON(s string, e *Entry) bool {
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return false
	}
	for k, v := range raw {
		sv := stringify(v)
		switch strings.ToLower(k) {
		case "time", "timestamp", "ts", "@timestamp":
			if e.Time.IsZero() {
				if t, ok := parseTime(sv); ok {
					e.Time = t
				}
			}
		case "level", "lvl", "severity":
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(sv)
			}
		case "msg", "message":
			if e.Message == "" {
				e.Message = sv
			}
		default:
			e.Fields[k] = sv
		}
	}
	if e.Message == "" {
		e.Message = s
	}
	return true
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func looksLikeLogfmt(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	c := s[eq-1]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-'
}

func parseLogfmt(s string, e *Entry) bool {
	fields := splitLogfmt(s)
	if len(fields) == 0 {
		return false
	}
	for k, v := range fields {
		switch strings.ToLower(k) {
		case "time", "timestamp", "ts":
			if e.Time.IsZero() {
				if t, ok := parseTime(v); ok {
					e.Time = t
				}
			}
		case "level", "lvl", "severity":
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(v)
			}
		case "msg", "message":
			if e.Message == "" {
				e.Message = v
			}
		default:
			e.Fields[k] = v
		}
	}
	if e.Message == "" {
		e.Message = s
	}
	return true
}

func splitLogfmt(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' {
			i++
		}
		if i >= len(s) || s[i] != '=' || i == start {
			return out
		}
		key := s[start:i]
		i++
		var val string
		if i < len(s) && s[i] == '"' {
			i++
			vs := i
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				i++
			}
			val = s[vs:i]
			if i < len(s) {
				i++
			}
		} else {
			vs := i
			for i < len(s) && s[i] != ' ' {
				i++
			}
			val = s[vs:i]
		}
		out[key] = val
	}
	return out
}

func parsePlain(s string, e *Entry) {
	lower := strings.ToLower(s)
	for _, kw := range []string{"error", "warn", "info", "debug", "trace", "fatal", "panic"} {
		if strings.Contains(lower, kw) {
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(kw)
			}
			break
		}
	}
	e.Message = s
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
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
