package parser

import (
	"regexp"
	"strings"
)

// codeFrameRe matches file/line references common in stack frames, e.g.
// "Foo.java:42", "main.go:10", "api.py:87". Used by the fallback rule of
// isContinuation to avoid swallowing arbitrary plain lines.
var codeFrameRe = regexp.MustCompile(`\.[A-Za-z]{1,4}:\d+`)

// LineAggregator merges continuation lines (stack traces, panics, exceptions)
// into the preceding Entry, so a multi-line log event is emitted as a single
// Entry with embedded newlines in Message and Raw.
type LineAggregator struct {
	pending   *Entry
	panicMode bool
	enabled   bool
}

// NewLineAggregator creates an aggregator. When enabled is false, Feed simply
// parses each line and returns it immediately (legacy per-line behavior).
func NewLineAggregator(enabled bool) *LineAggregator {
	return &LineAggregator{enabled: enabled}
}

// Feed processes one physical line. If a completed Entry is ready to be
// emitted (because this line starts a new event), it is returned with
// ready=true. Callers must also invoke Flush at EOF to drain the last entry.
func (a *LineAggregator) Feed(line string) (Entry, bool) {
	if !a.enabled {
		return Parse(line), true
	}
	if a.pending == nil {
		e := Parse(line)
		a.pending = &e
		a.panicMode = isPanicStart(line)
		return Entry{}, false
	}
	if isContinuation(line, a.pending) || (a.panicMode && isPanicContinuation(line)) {
		a.appendContinuation(line)
		return Entry{}, false
	}
	out := *a.pending
	e := Parse(line)
	a.pending = &e
	a.panicMode = isPanicStart(line)
	return out, true
}

func isPanicStart(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "panic:")
}

// isPanicContinuation is a permissive rule that only applies while we are
// aggregating a Go panic: any line that is not a recognized new event and
// does not have a leading timestamp is considered part of the panic dump.
func isPanicContinuation(line string) bool {
	if line == "" {
		return false
	}
	if _, _, ok := splitLeadingTimestamp(line); ok {
		return false
	}
	for _, p := range Parsers() {
		name := p.Name()
		if name == "plain" || name == "generic-timestamp" {
			continue
		}
		if p.Detect(line) {
			return false
		}
	}
	return true
}

// Flush returns and clears any pending entry.
func (a *LineAggregator) Flush() (Entry, bool) {
	if a.pending == nil {
		return Entry{}, false
	}
	out := *a.pending
	a.pending = nil
	a.panicMode = false
	return out, true
}

func (a *LineAggregator) appendContinuation(line string) {
	a.pending.Message = a.pending.Message + "\n" + line
	a.pending.Raw = a.pending.Raw + "\n" + line
	// Upgrade level when a continuation line hints at a panic/exception.
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "panic:"), strings.HasPrefix(trimmed, "goroutine "):
		if a.pending.Level < LevelFatal {
			a.pending.Level = LevelFatal
		}
	case a.pending.Level == LevelUnknown && (strings.HasPrefix(trimmed, "at ") ||
		strings.HasPrefix(trimmed, "Caused by:") ||
		strings.HasPrefix(trimmed, "Traceback") ||
		strings.Contains(trimmed, "Exception") ||
		strings.Contains(trimmed, "Error")):
		a.pending.Level = LevelError
	}
}

// isContinuation reports whether line should be merged into prev.
func isContinuation(line string, prev *Entry) bool {
	if prev == nil || line == "" {
		return false
	}
	if line[0] == ' ' || line[0] == '\t' {
		return true
	}
	switch {
	case strings.HasPrefix(line, "at "),
		strings.HasPrefix(line, "Caused by:"),
		strings.HasPrefix(line, "... "),
		strings.HasPrefix(line, "goroutine "),
		strings.HasPrefix(line, "panic:"):
		return true
	}
	// Fallback: exception-like text or a code frame with file:line reference,
	// with no leading timestamp and no non-generic parser matching. This is
	// deliberately conservative to avoid swallowing regular plain log lines.
	if _, _, ok := splitLeadingTimestamp(line); ok {
		return false
	}
	for _, p := range Parsers() {
		name := p.Name()
		if name == "plain" || name == "generic-timestamp" {
			continue
		}
		if p.Detect(line) {
			return false
		}
	}
	if codeFrameRe.MatchString(line) {
		return true
	}
	if strings.Contains(line, "Exception") || strings.Contains(line, "Traceback") {
		return true
	}
	// Python-style trailing exception like "KeyError: 42" or "ValueError: ...".
	if pyErrorRe.MatchString(line) {
		return true
	}
	return false
}

var pyErrorRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*Error:`)
