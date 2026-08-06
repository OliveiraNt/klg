package parser

import (
	"strings"
	"time"
)

// syslogParser handles RFC3164 (BSD) and RFC5424 syslog lines.
//
//	RFC3164: <34>Oct 11 22:14:15 mymachine su: 'su root' failed for lonvick
//	RFC5424: <34>1 2003-10-11T22:14:15.003Z mymachine.example.com su - ID47 - BOM'su root' failed
//
// Manual, regex-free scanner equivalent to the original patterns:
//
//	5424: ^<\d+>\d\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:\[[^\]]*\]|-)\s*(.*)$
//	3164: ^(?:<\d+>)?([A-Z][a-z]{2}\s+\d{1,2} \d{2}:\d{2}:\d{2})\s+(\S+)\s+
//	      ([^:\[]+)(?:\[(\d+)\])?:\s*(.*)$
type syslogParser struct{}

func (syslogParser) Name() string { return "syslog" }

func (syslogParser) Detect(line string) bool {
	s := trimSpace(line)
	if _, ok := scanSyslog5424(s); ok {
		return true
	}
	_, ok := scanSyslog3164(s)
	return ok
}

func (syslogParser) Parse(line string, e *Entry) bool {
	s := trimSpace(line)
	if f, ok := scanSyslog5424(s); ok {
		if e.Time.IsZero() {
			if t, ok := parseTime(f.ts); ok {
				e.Time = t
			}
		}
		e.Fields["host"] = f.host
		e.Fields["app"] = f.app
		if f.pid != "-" {
			e.Fields["pid"] = f.pid
		}
		if f.msgid != "-" {
			e.Fields["msgid"] = f.msgid
		}
		e.Message = f.message
		return true
	}
	f, ok := scanSyslog3164(s)
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.ts); ok {
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			e.Time = t
		}
	}
	e.Fields["host"] = f.host
	e.Fields["app"] = strings.TrimSpace(f.app)
	if f.pid != "" {
		e.Fields["pid"] = f.pid
	}
	e.Message = f.message
	return true
}

type syslog5424Fields struct {
	ts, host, app, pid, msgid, message string
}

// matchPRI matches `<\d+>` at i and returns the index after '>' (or -1).
func matchPRI(s string, i int) int {
	if i >= len(s) || s[i] != '<' {
		return -1
	}
	j := i + 1
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j == i+1 || j >= len(s) || s[j] != '>' {
		return -1
	}
	return j + 1
}

func scanSyslog5424(s string) (syslog5424Fields, bool) {
	var f syslog5424Fields
	i := matchPRI(s, 0)
	if i < 0 {
		return f, false
	}
	// version: a single digit.
	if i >= len(s) || !isDigit(s[i]) {
		return f, false
	}
	i++

	// five \S+ tokens separated by \s+.
	tokens := [5]string{}
	for t := range 5 {
		n := skipSpaces(s, i)
		if n == i {
			return f, false
		}
		start, end, ok := readToken(s, n)
		if !ok {
			return f, false
		}
		tokens[t] = s[start:end]
		i = end
	}
	f.ts, f.host, f.app, f.pid, f.msgid = tokens[0], tokens[1], tokens[2], tokens[3], tokens[4]

	// \s+ then structured data: `\[[^\]]*\]` or `-`.
	n := skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n
	switch {
	case i < len(s) && s[i] == '[':
		k := strings.IndexByte(s[i+1:], ']')
		if k < 0 {
			return f, false
		}
		i = i + 1 + k + 1
	case i < len(s) && s[i] == '-':
		i++
	default:
		return f, false
	}
	i = skipSpaces(s, i)
	f.message = s[i:]
	return f, true
}

type syslog3164Fields struct {
	ts, host, app, pid, message string
}

func scanSyslog3164(s string) (syslog3164Fields, bool) {
	var f syslog3164Fields
	i := 0
	if p := matchPRI(s, 0); p >= 0 {
		i = p
	}

	// timestamp: [A-Z][a-z]{2}\s+\d{1,2} \d{2}:\d{2}:\d{2}
	tsStart := i
	if i+3 > len(s) || !isUpperLetter(s[i]) ||
		!(s[i+1] >= 'a' && s[i+1] <= 'z') || !(s[i+2] >= 'a' && s[i+2] <= 'z') {
		return f, false
	}
	i += 3
	n := skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n
	// day: 1 or 2 digits (greedy).
	if i >= len(s) || !isDigit(s[i]) {
		return f, false
	}
	i++
	if i < len(s) && isDigit(s[i]) {
		i++
	}
	// literal single space then HH:MM:SS.
	if i >= len(s) || s[i] != ' ' {
		return f, false
	}
	i++
	if !matchDigits(s, i, 2) || !matchByte(s, i+2, ':') || !matchDigits(s, i+3, 2) ||
		!matchByte(s, i+5, ':') || !matchDigits(s, i+6, 2) {
		return f, false
	}
	i += 8
	f.ts = s[tsStart:i]

	// \s+ host \s+
	n = skipSpaces(s, i)
	if n == i {
		return f, false
	}
	hs, he, ok := readToken(s, n)
	if !ok {
		return f, false
	}
	f.host = s[hs:he]
	n = skipSpaces(s, he)
	if n == he {
		return f, false
	}
	i = n

	// app: [^:\[]+ (>=1)
	appStart := i
	for i < len(s) && s[i] != ':' && s[i] != '[' {
		i++
	}
	if i == appStart {
		return f, false
	}
	f.app = s[appStart:i]

	// optional [pid]
	if i < len(s) && s[i] == '[' {
		j := i + 1
		for j < len(s) && isDigit(s[j]) {
			j++
		}
		if j > i+1 && j < len(s) && s[j] == ']' {
			f.pid = s[i+1 : j]
			i = j + 1
		}
	}

	// literal ':' then \s* then message.
	if i >= len(s) || s[i] != ':' {
		return f, false
	}
	i++
	i = skipSpaces(s, i)
	f.message = s[i:]
	return f, true
}

func init() { Register(syslogParser{}, 70) }

// genericTimestampParser is a low-priority fallback: it fires when a line
// starts with a recognizable timestamp followed by free text and no other
// specific parser matched. It extracts the timestamp and leaves the rest as
// the message; a keyword scan picks up the level.
//
// Manual, regex-free scanner equivalent to:
//
//	^(\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?
//	 (?:Z|[+-]\d{2}:?\d{2})?)\s+(.*)$
type genericTimestampParser struct{}

var genericKeywords = []string{"error", "warn", "info", "debug", "trace", "fatal", "panic"}

func (genericTimestampParser) Name() string { return "generic-timestamp" }

func (genericTimestampParser) Detect(line string) bool {
	_, _, ok := scanGeneric(trimSpace(line))
	return ok
}

func (genericTimestampParser) Parse(line string, e *Entry) bool {
	ts, rest, ok := scanGeneric(trimSpace(line))
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(ts); ok {
			e.Time = t
		}
	}
	if e.Level == LevelUnknown {
		lower := strings.ToLower(rest)
		for _, kw := range genericKeywords {
			if strings.Contains(lower, kw) {
				e.Level = ParseLevel(kw)
				break
			}
		}
	}
	e.Message = rest
	return true
}

func scanGeneric(s string) (ts, rest string, ok bool) {
	// \d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}
	if len(s) < 19 || !matchDigits(s, 0, 4) || (s[4] != '-' && s[4] != '/') ||
		!matchDigits(s, 5, 2) || (s[7] != '-' && s[7] != '/') || !matchDigits(s, 8, 2) ||
		(s[10] != 'T' && s[10] != ' ') || !matchDigits(s, 11, 2) || s[13] != ':' ||
		!matchDigits(s, 14, 2) || s[16] != ':' || !matchDigits(s, 17, 2) {
		return "", "", false
	}
	i := 19
	// optional [.,]\d+
	if i < len(s) && (s[i] == '.' || s[i] == ',') && i+1 < len(s) && isDigit(s[i+1]) {
		i += 2
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	// optional Z | [+-]\d{2}:?\d{2}
	if i < len(s) {
		if s[i] == 'Z' {
			i++
		} else if s[i] == '+' || s[i] == '-' {
			if matchDigits(s, i+1, 2) {
				j := i + 3
				if j < len(s) && s[j] == ':' {
					j++
				}
				if matchDigits(s, j, 2) {
					i = j + 2
				}
			}
		}
	}
	ts = s[:i]
	// \s+ then (.*)
	n := skipSpaces(s, i)
	if n == i {
		return "", "", false
	}
	return ts, s[n:], true
}

func init() { Register(genericTimestampParser{}, 10) }
