// Package parser detecta e extrai campos comuns (timestamp, level, message)
// de linhas de log em vários formatos: JSON, logfmt e texto livre.
package parser

import (
	"encoding/json"
	"strings"
	"time"
)

// Level representa a severidade de um registro de log.
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

// String devolve o nome canônico do nível (maiúsculas).
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

// ParseLevel converte uma string arbitrária em Level.
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

// Entry é uma entrada de log normalizada.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  map[string]string
	Raw     string
}

// Parse tenta interpretar a linha como JSON, depois logfmt, e por fim texto livre.
func Parse(line string) Entry {
	e := Entry{Raw: line, Fields: map[string]string{}}
	trimmed := strings.TrimSpace(line)

	// kubectl pode prefixar com timestamp RFC3339 quando usado com --timestamps.
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
	// Heurística: contém pelo menos um par chave=valor sem espaço antes do '='.
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	// caractere anterior ao '=' precisa ser [a-zA-Z0-9_.-]
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

// splitLogfmt faz parsing simples de pares chave=valor com suporte a aspas.
func splitLogfmt(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		// chave
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' {
			i++
		}
		if i >= len(s) || s[i] != '=' || i == start {
			return out
		}
		key := s[start:i]
		i++ // consome '='
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
				i++ // consome '"' de fechamento
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
	// Tenta detectar nível dentro de colchetes ou como palavra inicial.
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
