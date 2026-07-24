package parser

import "sort"

// Parser is the extensibility contract for log-format parsers.
//
// Implementations should be cheap to Detect: it is invoked for every input line
// on every registered parser (in priority order) until one succeeds. Parse fills
// the provided Entry in place and reports whether parsing was successful.
//
// A new format can be added by creating a new file under internal/parser that
// defines a type implementing Parser and calls Register in its init function.
type Parser interface {
	// Name returns a short identifier used for diagnostics.
	Name() string
	// Detect reports whether the parser is likely able to handle the line.
	Detect(line string) bool
	// Parse tries to fill e from line. It returns true on success.
	Parse(line string, e *Entry) bool
}

type registered struct {
	p        Parser
	priority int
}

var registry []registered

// Register adds p to the parser registry with the given priority.
// Higher priority parsers are attempted first.
//
// Register is not safe for concurrent use; it is intended to be called from
// package init functions.
func Register(p Parser, priority int) {
	registry = append(registry, registered{p: p, priority: priority})
	sort.SliceStable(registry, func(i, j int) bool {
		return registry[i].priority > registry[j].priority
	})
}

// Parsers returns the registered parsers ordered by descending priority.
func Parsers() []Parser {
	out := make([]Parser, len(registry))
	for i, r := range registry {
		out[i] = r.p
	}
	return out
}

// dispatch iterates the registry and returns the first parser that succeeds.
func dispatch(line string, e *Entry) bool {
	for _, r := range registry {
		if !r.p.Detect(line) {
			continue
		}
		if r.p.Parse(line, e) {
			return true
		}
	}
	return false
}
