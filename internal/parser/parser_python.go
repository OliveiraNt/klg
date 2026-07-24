package parser

import (
	"regexp"
	"strings"
)

// pythonParser handles the Python `logging` default format:
//
//	2024-01-15 10:23:45,123 - my.logger - LEVEL - message
type pythonParser struct{}

var pythonRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:[,.]\d{3})?)\s-\s(\S+)\s-\s(DEBUG|INFO|WARNING|WARN|ERROR|CRITICAL|FATAL)\s-\s(.*)$`)

func (pythonParser) Name() string { return "python" }

func (pythonParser) Detect(line string) bool {
	return pythonRe.MatchString(strings.TrimSpace(line))
}

func (pythonParser) Parse(line string, e *Entry) bool {
	m := pythonRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[1]); ok {
			e.Time = t
		}
	}
	e.Fields["logger"] = m[2]
	if e.Level == LevelUnknown {
		e.Level = ParseLevel(m[3])
	}
	e.Message = m[4]
	return true
}

func init() { Register(pythonParser{}, 60) }
