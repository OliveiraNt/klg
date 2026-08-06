package parser

// pythonParser handles the Python `logging` default format:
//
//	2024-01-15 10:23:45,123 - my.logger - LEVEL - message
//
// It is a manual, regex-free scanner equivalent to the original pattern:
//
//	^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:[,.]\d{3})?)\s-\s(\S+)\s-\s
//	 (DEBUG|INFO|WARNING|WARN|ERROR|CRITICAL|FATAL)\s-\s(.*)$
type pythonParser struct{}

var pythonLevels = []string{"DEBUG", "INFO", "WARNING", "WARN", "ERROR", "CRITICAL", "FATAL"}

func (pythonParser) Name() string { return "python" }

func (pythonParser) Detect(line string) bool {
	_, ok := scanPython(trimSpace(line))
	return ok
}

func (pythonParser) Parse(line string, e *Entry) bool {
	f, ok := scanPython(trimSpace(line))
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.ts); ok {
			e.Time = t
		}
	}
	e.Fields["logger"] = f.logger
	if e.Level == LevelUnknown {
		e.Level = ParseLevel(f.level)
	}
	e.Message = f.message
	return true
}

type pythonFields struct {
	ts, logger, level, message string
}

func scanPython(s string) (pythonFields, bool) {
	var f pythonFields
	n := matchDateTime(s)
	if n < 0 {
		return f, false
	}
	end := matchFraction(s, n, false)
	f.ts = s[:end]

	i := matchSep(s, end)
	if i < 0 {
		return f, false
	}

	// logger: one or more non-space characters.
	start := i
	for i < len(s) && !isASCIISpace(s[i]) {
		i++
	}
	if i == start {
		return f, false
	}
	f.logger = s[start:i]

	i = matchSep(s, i)
	if i < 0 {
		return f, false
	}

	// level: a fixed keyword immediately followed by a separator.
	lvlEnd := -1
	for _, kw := range pythonLevels {
		if len(s)-i >= len(kw) && s[i:i+len(kw)] == kw {
			if sep := matchSep(s, i+len(kw)); sep >= 0 {
				f.level = kw
				lvlEnd = sep
				break
			}
		}
	}
	if lvlEnd < 0 {
		return f, false
	}
	f.message = s[lvlEnd:]
	return f, true
}

func init() { Register(pythonParser{}, 60) }
