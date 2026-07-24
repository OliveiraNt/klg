package parser

import (
	"regexp"
	"strings"
)

// nginxAccessParser handles the Nginx/Apache combined access log format:
//
//	127.0.0.1 - user [10/Oct/2000:13:55:36 -0700] "GET /path HTTP/1.0" 200 2326 "-" "UA"
type nginxAccessParser struct{}

var accessRe = regexp.MustCompile(`^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"\s+(\d{3})\s+(\S+)(?:\s+"([^"]*)"\s+"([^"]*)")?`)

func (nginxAccessParser) Name() string { return "nginx-access" }

func (nginxAccessParser) Detect(line string) bool {
	return accessRe.MatchString(strings.TrimSpace(line))
}

func (nginxAccessParser) Parse(line string, e *Entry) bool {
	return parseAccess(line, e)
}

func parseAccess(line string, e *Entry) bool {
	m := accessRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(m[3]); ok {
			e.Time = t
		}
	}
	e.Fields["remote"] = m[1]
	if m[2] != "-" {
		e.Fields["user"] = m[2]
	}
	e.Fields["method"] = m[4]
	e.Fields["path"] = m[5]
	e.Fields["status"] = m[6]
	e.Fields["size"] = m[7]
	if len(m) > 8 && m[8] != "" && m[8] != "-" {
		e.Fields["referer"] = m[8]
	}
	if len(m) > 9 && m[9] != "" && m[9] != "-" {
		e.Fields["user_agent"] = m[9]
	}
	// Derive a level from HTTP status: 5xx=error, 4xx=warn, else info.
	if e.Level == LevelUnknown && len(m[6]) == 3 {
		switch m[6][0] {
		case '5':
			e.Level = LevelError
		case '4':
			e.Level = LevelWarn
		default:
			e.Level = LevelInfo
		}
	}
	e.Message = m[4] + " " + m[5] + " " + m[6]
	return true
}

func init() { Register(nginxAccessParser{}, 80) }

// nginxErrorParser handles Nginx error log lines:
//
//	2024/01/15 10:23:45 [error] 1234#0: *5 open() failed ...
type nginxErrorParser struct{}

var nginxErrRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+\[(debug|info|notice|warn|error|crit|alert|emerg)\]\s+(\d+#\d+):\s*(.*)$`)

func (nginxErrorParser) Name() string { return "nginx-error" }

func (nginxErrorParser) Detect(line string) bool {
	return nginxErrRe.MatchString(strings.TrimSpace(line))
}

func (nginxErrorParser) Parse(line string, e *Entry) bool {
	m := nginxErrRe.FindStringSubmatch(strings.TrimSpace(line))
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
	e.Fields["pid"] = m[3]
	e.Message = m[4]
	return true
}

func init() { Register(nginxErrorParser{}, 75) }
