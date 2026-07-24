package parser

import (
	"regexp"
	"strings"
	"time"
)

// syslogParser handles RFC3164 (BSD) and RFC5424 syslog lines.
//
//	RFC3164: <34>Oct 11 22:14:15 mymachine su: 'su root' failed for lonvick
//	RFC5424: <34>1 2003-10-11T22:14:15.003Z mymachine.example.com su - ID47 - BOM'su root' failed
type syslogParser struct{}

var (
	syslog3164Re = regexp.MustCompile(`^(?:<\d+>)?([A-Z][a-z]{2}\s+\d{1,2} \d{2}:\d{2}:\d{2})\s+(\S+)\s+([^:\[]+)(?:\[(\d+)\])?:\s*(.*)$`)
	syslog5424Re = regexp.MustCompile(`^<\d+>\d\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:\[[^\]]*\]|-)\s*(.*)$`)
)

func (syslogParser) Name() string { return "syslog" }

func (syslogParser) Detect(line string) bool {
	s := strings.TrimSpace(line)
	return syslog5424Re.MatchString(s) || syslog3164Re.MatchString(s)
}

func (syslogParser) Parse(line string, e *Entry) bool {
	s := strings.TrimSpace(line)
	if m := syslog5424Re.FindStringSubmatch(s); m != nil {
		if e.Time.IsZero() {
			if t, ok := parseTime(m[1]); ok {
				e.Time = t
			}
		}
		e.Fields["host"] = m[2]
		e.Fields["app"] = m[3]
		if m[4] != "-" {
			e.Fields["pid"] = m[4]
		}
		if m[5] != "-" {
			e.Fields["msgid"] = m[5]
		}
		e.Message = m[6]
		return true
	}
	m := syslog3164Re.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[1]); ok {
			// RFC3164 omits the year; assume current year.
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			e.Time = t
		}
	}
	e.Fields["host"] = m[2]
	e.Fields["app"] = strings.TrimSpace(m[3])
	if m[4] != "" {
		e.Fields["pid"] = m[4]
	}
	e.Message = m[5]
	return true
}

func init() { Register(syslogParser{}, 70) }

// genericTimestampParser is a low-priority fallback: it fires when a line
// starts with a recognizable timestamp followed by free text and no other
// specific parser matched. It extracts the timestamp and leaves the rest as
// the message; a keyword scan picks up the level.
type genericTimestampParser struct{}

var genericTsRe = regexp.MustCompile(`^(\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+(.*)$`)

func (genericTimestampParser) Name() string { return "generic-timestamp" }

func (genericTimestampParser) Detect(line string) bool {
	return genericTsRe.MatchString(strings.TrimSpace(line))
}

func (genericTimestampParser) Parse(line string, e *Entry) bool {
	m := genericTsRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[1]); ok {
			e.Time = t
		}
	}
	rest := m[2]
	lower := strings.ToLower(rest)
	for _, kw := range []string{"error", "warn", "info", "debug", "trace", "fatal", "panic"} {
		if strings.Contains(lower, kw) {
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(kw)
			}
			break
		}
	}
	e.Message = rest
	return true
}

func init() { Register(genericTimestampParser{}, 10) }
