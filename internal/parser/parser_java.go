package parser

import (
	"regexp"
	"strings"
)

// javaParser handles Log4j/Logback default patterns like:
//
//	2024-01-15 10:23:45,123 INFO  [thread-1] com.foo.Bar - message
type javaParser struct{}

var javaRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[,.]\d{3})\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\b\s*(?:\[([^\]]+)\])?\s*([\w.$]+)?\s*[-:]?\s*(.*)$`)

func (javaParser) Name() string { return "java" }

func (javaParser) Detect(line string) bool {
	return javaRe.MatchString(strings.TrimSpace(line))
}

func (javaParser) Parse(line string, e *Entry) bool {
	m := javaRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[1]); ok {
			e.Time = t
		}
	}
	if e.Level == LevelUnknown {
		e.Level = ParseLevel(m[2])
	}
	if m[3] != "" {
		e.Fields["thread"] = m[3]
	}
	if m[4] != "" {
		e.Fields["logger"] = m[4]
	}
	e.Message = strings.TrimSpace(m[5])
	return true
}

func init() { Register(javaParser{}, 65) }
