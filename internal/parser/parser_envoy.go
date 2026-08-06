package parser

import "strings"

// envoyParser handles Envoy's default access log format:
//
//	[2024-01-15T10:23:45.123Z] "GET /path HTTP/1.1" 200 - 0 1234 5 4 "-" "curl/8" ...
//
// Manual, regex-free scanner equivalent to:
//
//	^\[(\d{4}-\d{2}-\d{2}T[\d:.Z+-]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"
//	 \s+(\d{3})\s+(\S+)
type envoyParser struct{}

func (envoyParser) Name() string { return "envoy" }

func (envoyParser) Detect(line string) bool {
	_, ok := scanEnvoy(trimSpace(line))
	return ok
}

func (envoyParser) Parse(line string, e *Entry) bool {
	f, ok := scanEnvoy(trimSpace(line))
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.time); ok {
			e.Time = t
		}
	}
	e.Fields["method"] = f.method
	e.Fields["path"] = f.path
	e.Fields["status"] = f.status
	e.Fields["response_flags"] = f.flags
	if e.Level == LevelUnknown && len(f.status) == 3 {
		switch f.status[0] {
		case '5':
			e.Level = LevelError
		case '4':
			e.Level = LevelWarn
		default:
			e.Level = LevelInfo
		}
	}
	e.Message = f.method + " " + f.path + " " + f.status
	return true
}

type envoyFields struct {
	time, method, path, status, flags string
}

// isEnvoyTSByte reports whether b belongs to the timestamp tail character
// class [\d:.Z+-].
func isEnvoyTSByte(b byte) bool {
	return isDigit(b) || b == ':' || b == '.' || b == 'Z' || b == '+' || b == '-'
}

func scanEnvoy(s string) (envoyFields, bool) {
	var f envoyFields
	if len(s) == 0 || s[0] != '[' {
		return f, false
	}
	j := strings.IndexByte(s, ']')
	if j < 0 {
		return f, false
	}
	inner := s[1:j]
	// \d{4}-\d{2}-\d{2}T[\d:.Z+-]+
	if len(inner) < 12 || !matchDigits(inner, 0, 4) || inner[4] != '-' ||
		!matchDigits(inner, 5, 2) || inner[7] != '-' || !matchDigits(inner, 8, 2) ||
		inner[10] != 'T' {
		return f, false
	}
	for x := 11; x < len(inner); x++ {
		if !isEnvoyTSByte(inner[x]) {
			return f, false
		}
	}
	f.time = inner
	i := skipSpaces(s, j+1)
	if i == j+1 {
		return f, false
	}

	// "METHOD PATH HTTP/x.y"
	if i >= len(s) || s[i] != '"' {
		return f, false
	}
	q := strings.IndexByte(s[i+1:], '"')
	if q < 0 {
		return f, false
	}
	var af accessFields
	if !scanRequest(s[i+1:i+1+q], &af) {
		return f, false
	}
	f.method = af.method
	f.path = af.path
	i = i + 1 + q + 1

	// \s+ status(3 digits) \s+ flags(\S+)
	n := skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n
	if !matchDigits(s, i, 3) || i+3 >= len(s) || !isASCIISpace(s[i+3]) {
		return f, false
	}
	f.status = s[i : i+3]
	i = skipSpaces(s, i+3)
	fs, fe, ok := readToken(s, i)
	if !ok {
		return f, false
	}
	f.flags = s[fs:fe]
	return f, true
}

func init() { Register(envoyParser{}, 80) }
