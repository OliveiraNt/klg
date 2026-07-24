package parser

import "strings"

type logfmtParser struct{}

func (logfmtParser) Name() string { return "logfmt" }

func (logfmtParser) Detect(line string) bool {
	s := strings.TrimSpace(line)
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	c := s[eq-1]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-'
}

func (logfmtParser) Parse(line string, e *Entry) bool {
	s := strings.TrimSpace(line)
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

func splitLogfmt(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' {
			i++
		}
		if i >= len(s) || s[i] != '=' || i == start {
			return out
		}
		key := s[start:i]
		i++
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
				i++
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

func init() { Register(logfmtParser{}, 50) }
