package parser

import "strings"

// Manual (regex-free) scanning helpers shared by the format parsers. They all
// operate on string indices and never allocate.

// matchDigits reports whether s[i:i+n] are all ASCII digits.
func matchDigits(s string, i, n int) bool {
	if i < 0 || i+n > len(s) {
		return false
	}
	for j := i; j < i+n; j++ {
		if !isDigit(s[j]) {
			return false
		}
	}
	return true
}

// matchByte reports whether s has b at position i.
func matchByte(s string, i int, b byte) bool {
	return i < len(s) && s[i] == b
}

// matchDateTime matches the fixed prefix `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`
// (a "YYYY-MM-DD HH:MM:SS" stamp) and returns its length (19) or -1.
func matchDateTime(s string) int {
	if len(s) < 19 {
		return -1
	}
	if !matchDigits(s, 0, 4) || s[4] != '-' || !matchDigits(s, 5, 2) || s[7] != '-' ||
		!matchDigits(s, 8, 2) || s[10] != ' ' || !matchDigits(s, 11, 2) || s[13] != ':' ||
		!matchDigits(s, 14, 2) || s[16] != ':' || !matchDigits(s, 17, 2) {
		return -1
	}
	return 19
}

// matchDateTimeSlash matches the fixed prefix
// `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}` (Nginx error "YYYY/MM/DD HH:MM:SS")
// and returns its length (19) or -1.
func matchDateTimeSlash(s string) int {
	if len(s) < 19 {
		return -1
	}
	if !matchDigits(s, 0, 4) || s[4] != '/' || !matchDigits(s, 5, 2) || s[7] != '/' ||
		!matchDigits(s, 8, 2) || s[10] != ' ' || !matchDigits(s, 11, 2) || s[13] != ':' ||
		!matchDigits(s, 14, 2) || s[16] != ':' || !matchDigits(s, 17, 2) {
		return -1
	}
	return 19
}

// matchFraction matches an optional `[,.]\d{3}` starting at i and returns the
// new index. required controls whether the fractional part must be present:
// it returns -1 when required and absent/malformed.
func matchFraction(s string, i int, required bool) int {
	if i+4 <= len(s) && (s[i] == ',' || s[i] == '.') && matchDigits(s, i+1, 3) {
		return i + 4
	}
	if required {
		return -1
	}
	return i
}

// matchSep matches a `\s-\s` separator (whitespace, '-', whitespace) at i and
// returns the index after it, or -1.
func matchSep(s string, i int) int {
	if i+3 <= len(s) && isASCIISpace(s[i]) && s[i+1] == '-' && isASCIISpace(s[i+2]) {
		return i + 3
	}
	return -1
}

// containsCodeFrame reports whether s contains a stack-frame file reference,
// matching the regex `\.[A-Za-z]{1,4}:\d+` (e.g. "Foo.java:42", "main.go:10").
func containsCodeFrame(s string) bool {
	i := 0
	for i < len(s) {
		d := strings.IndexByte(s[i:], '.')
		if d < 0 {
			return false
		}
		p := i + d
		j := p + 1
		for j < len(s) && j-(p+1) < 4 && isLetter(s[j]) {
			j++
		}
		if j > p+1 && j < len(s) && s[j] == ':' && j+1 < len(s) && isDigit(s[j+1]) {
			return true
		}
		i = p + 1
	}
	return false
}

// matchPyError reports whether s starts with a Python-style exception name,
// matching the regex `^[A-Z][A-Za-z0-9_]*Error:` (e.g. "KeyError: 42").
func matchPyError(s string) bool {
	if len(s) == 0 || !isUpperLetter(s[0]) {
		return false
	}
	end := 1
	for end < len(s) && isWordByte(s[end]) {
		end++
	}
	// Need at least one leading char before the trailing "Error", i.e. the
	// [A-Z] plus "Error" => 6 word chars, immediately followed by ':'.
	if end < 6 || end >= len(s) || s[end] != ':' {
		return false
	}
	return s[end-5:end] == "Error"
}

// skipSpaces returns the first index >= i that is not an ASCII space run of
// s (matching \s+). It advances over the whitespace bytes handled by
// isASCIISpace.
func skipSpaces(s string, i int) int {
	for i < len(s) && isASCIISpace(s[i]) {
		i++
	}
	return i
}
