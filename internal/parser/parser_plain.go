package parser

import "strings"

type plainParser struct{}

func (plainParser) Name() string { return "plain" }

// Detect always returns true: plain acts as a catch-all fallback.
func (plainParser) Detect(string) bool { return true }

func (plainParser) Parse(line string, e *Entry) bool {
	s := strings.TrimSpace(line)
	lower := strings.ToLower(s)
	for _, kw := range []string{"error", "warn", "info", "debug", "trace", "fatal", "panic"} {
		if strings.Contains(lower, kw) {
			if e.Level == LevelUnknown {
				e.Level = ParseLevel(kw)
			}
			break
		}
	}
	e.Message = s
	return true
}

func init() { Register(plainParser{}, 0) }
