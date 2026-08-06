package parser

import (
	"encoding/json"
	"strings"
)

type jsonParser struct{}

func (jsonParser) Name() string { return "json" }

func (jsonParser) Detect(line string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func (jsonParser) Parse(line string, e *Entry) bool {
	s := strings.TrimSpace(line)
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return false
	}
	for k, v := range raw {
		sv := stringify(v)
		switch {
		case equalFoldASCII(k, "time"), equalFoldASCII(k, "timestamp"),
			equalFoldASCII(k, "ts"), equalFoldASCII(k, "@timestamp"):
			if e.Time.IsZero() {
				if t, ok := parseTime(sv); ok {
					e.Time = t
				}
			}
		case equalFoldASCII(k, "level"), equalFoldASCII(k, "lvl"), equalFoldASCII(k, "severity"):
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(sv)
			}
		case equalFoldASCII(k, "msg"), equalFoldASCII(k, "message"):
			if e.Message == "" {
				e.Message = sv
			}
		default:
			e.Fields[k] = sv
		}
	}
	if e.Message == "" {
		e.Message = s
	}
	return true
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func init() { Register(jsonParser{}, 100) }
