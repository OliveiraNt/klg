package parser

import "strings"

// javaParser handles Log4j/Logback default patterns like:
//
//	2024-01-15 10:23:45,123 INFO  [thread-1] com.foo.Bar - message
//
// It is a manual, regex-free scanner equivalent to the original pattern:
//
//	^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[,.]\d{3})\s+
//	 (TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\b\s*(?:\[([^\]]+)\])?\s*
//	 ([\w.$]+)?\s*[-:]?\s*(.*)$
type javaParser struct{}

var javaLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

func (javaParser) Name() string { return "java" }

func (javaParser) Detect(line string) bool {
	_, ok := scanJava(trimSpace(line))
	return ok
}

func (javaParser) Parse(line string, e *Entry) bool {
	f, ok := scanJava(trimSpace(line))
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.ts); ok {
			e.Time = t
		}
	}
	if e.Level == LevelUnknown {
		e.Level = ParseLevel(f.level)
	}
	if f.thread != "" {
		e.Fields["thread"] = f.thread
	}
	if f.logger != "" {
		e.Fields["logger"] = f.logger
	}
	e.Message = f.message
	return true
}

type javaFields struct {
	ts, level, thread, logger, message string
}

func scanJava(s string) (javaFields, bool) {
	var f javaFields
	n := matchDateTime(s)
	if n < 0 {
		return f, false
	}
	end := matchFraction(s, n, true) // Java requires the ",123"/".123" fraction.
	if end < 0 {
		return f, false
	}
	f.ts = s[:end]

	// \s+ (at least one whitespace) before the level.
	i := skipSpaces(s, end)
	if i == end {
		return f, false
	}

	// level keyword, followed by a \b word boundary.
	lvlEnd := -1
	for _, kw := range javaLevels {
		if len(s)-i >= len(kw) && s[i:i+len(kw)] == kw {
			after := i + len(kw)
			if after >= len(s) || !isWordByte(s[after]) {
				f.level = kw
				lvlEnd = after
				break
			}
		}
	}
	if lvlEnd < 0 {
		return f, false
	}
	i = skipSpaces(s, lvlEnd)

	// Optional "[thread]".
	if i < len(s) && s[i] == '[' {
		if k := strings.IndexByte(s[i+1:], ']'); k >= 1 {
			f.thread = s[i+1 : i+1+k]
			i = i + 1 + k + 1
		}
	}
	i = skipSpaces(s, i)

	// Optional logger token ([\w.$]+).
	start := i
	for i < len(s) && isLoggerByte(s[i]) {
		i++
	}
	if i > start {
		f.logger = s[start:i]
	}
	i = skipSpaces(s, i)

	// Optional single [-:] separator.
	if i < len(s) && (s[i] == '-' || s[i] == ':') {
		i++
	}

	f.message = trimSpace(s[i:])
	return f, true
}

func init() { Register(javaParser{}, 65) }
