package parser

import (
	"regexp"
	"strings"
)

// envoyParser handles Envoy's default access log format:
//
//	[2024-01-15T10:23:45.123Z] "GET /path HTTP/1.1" 200 - 0 1234 5 4 "-" "curl/8" "req-id" "example.com" "10.0.0.1:8080"
type envoyParser struct{}

var envoyRe = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}T[\d:.Z+-]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"\s+(\d{3})\s+(\S+)`)

func (envoyParser) Name() string { return "envoy" }

func (envoyParser) Detect(line string) bool {
	return envoyRe.MatchString(strings.TrimSpace(line))
}

func (envoyParser) Parse(line string, e *Entry) bool {
	m := envoyRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[1]); ok {
			e.Time = t
		}
	}
	e.Fields["method"] = m[2]
	e.Fields["path"] = m[3]
	e.Fields["status"] = m[4]
	e.Fields["response_flags"] = m[5]
	if e.Level == LevelUnknown && len(m[4]) == 3 {
		switch m[4][0] {
		case '5':
			e.Level = LevelError
		case '4':
			e.Level = LevelWarn
		default:
			e.Level = LevelInfo
		}
	}
	e.Message = m[2] + " " + m[3] + " " + m[4]
	return true
}

func init() { Register(envoyParser{}, 80) }
