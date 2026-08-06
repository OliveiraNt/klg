package parser

// This file contains small, allocation-free ASCII helpers shared by the
// manual (regex-free) scanners. They intentionally avoid strings.ToLower /
// strings.TrimSpace over freshly allocated strings so hot parsing paths do
// not allocate.

// equalFoldASCII reports whether s equals lower, comparing case-insensitively
// for ASCII letters. lower MUST already be lowercase; this is an internal
// contract used with string literals. It performs no allocation.
func equalFoldASCII(s, lower string) bool {
	if len(s) != len(lower) {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}
	return true
}

// trimSpace trims leading and trailing ASCII whitespace, returning a substring
// (no allocation). For the fully-ASCII log inputs handled here it is equivalent
// to strings.TrimSpace while avoiding its Unicode-aware slow path.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && isASCIISpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isASCIISpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isASCIISpace reports whether b is one of the ASCII whitespace bytes handled
// by strings.TrimSpace's ASCII fast path (space, tab, newline, CR, VT, FF).
func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

// isDigit reports whether b is an ASCII decimal digit.
func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// isWordByte reports whether b is a regex \w character ([0-9A-Za-z_]).
func isWordByte(b byte) bool {
	return b == '_' || isDigit(b) || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isLoggerByte reports whether b belongs to a Java logger token ([\w.$]).
func isLoggerByte(b byte) bool { return isWordByte(b) || b == '.' || b == '$' }

// isLetter reports whether b is an ASCII letter ([A-Za-z]).
func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// isUpperLetter reports whether b is an uppercase ASCII letter (e.g. the
// first letter of an English month-name abbreviation such as "Oct").
func isUpperLetter(b byte) bool { return b >= 'A' && b <= 'Z' }
