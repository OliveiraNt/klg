package parser

import "strings"

// nginxAccessParser handles the Nginx/Apache combined access log format:
//
//	127.0.0.1 - user [10/Oct/2000:13:55:36 -0700] "GET /path HTTP/1.0" 200 2326 "-" "UA"
//
// Manual, regex-free scanner equivalent to:
//
//	^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"
//	 \s+(\d{3})\s+(\S+)(?:\s+"([^"]*)"\s+"([^"]*)")?
type nginxAccessParser struct{}

func (nginxAccessParser) Name() string { return "nginx-access" }

func (nginxAccessParser) Detect(line string) bool {
	_, ok := scanAccess(trimSpace(line))
	return ok
}

func (nginxAccessParser) Parse(line string, e *Entry) bool {
	return parseAccess(trimSpace(line), e)
}

type accessFields struct {
	remote, user, time, method, path, status, size, referer, userAgent string
	hasReferer                                                         bool
}

func parseAccess(s string, e *Entry) bool {
	f, ok := scanAccess(s)
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.time); ok {
			e.Time = t
		}
	}
	e.Fields["remote"] = f.remote
	if f.user != "-" {
		e.Fields["user"] = f.user
	}
	e.Fields["method"] = f.method
	e.Fields["path"] = f.path
	e.Fields["status"] = f.status
	e.Fields["size"] = f.size
	if f.hasReferer {
		if f.referer != "" && f.referer != "-" {
			e.Fields["referer"] = f.referer
		}
		if f.userAgent != "" && f.userAgent != "-" {
			e.Fields["user_agent"] = f.userAgent
		}
	}
	if e.Level == LevelUnknown && len(f.status) == 3 {
		switch f.status[0] {
		case '5':
			e.Level = LevelError
		case '4':
			e.Level = LevelWarn
		default:
			e.Level = LevelInfo
		}
	}
	e.Message = f.method + " " + f.path + " " + f.status
	return true
}

// readToken reads a \S+ token at i and returns its bounds and the index after
// it; ok is false when the token is empty.
func readToken(s string, i int) (start, end int, ok bool) {
	start = i
	for i < len(s) && !isASCIISpace(s[i]) {
		i++
	}
	return start, i, i > start
}

func scanAccess(s string) (accessFields, bool) {
	var f accessFields

	// remote \s+ ident \s+ user \s+
	_, e1, ok := readToken(s, 0)
	if !ok {
		return f, false
	}
	f.remote = s[:e1]
	i := skipSpaces(s, e1)
	if i == e1 {
		return f, false
	}

	_, ie, ok := readToken(s, i) // ident (ignored)
	if !ok {
		return f, false
	}
	j := skipSpaces(s, ie)
	if j == ie {
		return f, false
	}

	us, ue, ok := readToken(s, j) // user
	if !ok {
		return f, false
	}
	f.user = s[us:ue]
	i = skipSpaces(s, ue)
	if i == ue {
		return f, false
	}

	// [time]
	if i >= len(s) || s[i] != '[' {
		return f, false
	}
	k := strings.IndexByte(s[i+1:], ']')
	if k < 1 {
		return f, false
	}
	f.time = s[i+1 : i+1+k]
	i = i + 1 + k + 1
	n := skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n

	// "METHOD PATH HTTP/x.y"
	if i >= len(s) || s[i] != '"' {
		return f, false
	}
	q := strings.IndexByte(s[i+1:], '"')
	if q < 0 {
		return f, false
	}
	rc := s[i+1 : i+1+q]
	if !scanRequest(rc, &f) {
		return f, false
	}
	i = i + 1 + q + 1

	// \s+ status(3 digits) \s+ size(\S+)
	n = skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n
	if !matchDigits(s, i, 3) || (i+3 < len(s) && !isASCIISpace(s[i+3])) {
		return f, false
	}
	if i+3 > len(s) {
		return f, false
	}
	f.status = s[i : i+3]
	i += 3
	n = skipSpaces(s, i)
	if n == i {
		return f, false
	}
	i = n
	ss, se, ok := readToken(s, i)
	if !ok {
		return f, false
	}
	f.size = s[ss:se]
	i = se

	// Optional: \s+ "referer" \s+ "user_agent"
	scanOptionalQuotes(s, i, &f)
	return f, true
}

// scanRequest parses `METHOD PATH HTTP/x.y` (the content between the request
// quotes) into method and path.
func scanRequest(rc string, f *accessFields) bool {
	mEnd := strings.IndexByte(rc, ' ')
	if mEnd < 1 {
		return false
	}
	for x := range mEnd {
		if rc[x] < 'A' || rc[x] > 'Z' {
			return false
		}
	}
	f.method = rc[:mEnd]

	hi := strings.LastIndex(rc, "HTTP/")
	if hi < 0 {
		return false
	}
	// HTTP/[\d.]+ must run to the end of the request content.
	ver := rc[hi+5:]
	if ver == "" {
		return false
	}
	for x := range len(ver) {
		if !isDigit(ver[x]) && ver[x] != '.' {
			return false
		}
	}
	// There must be whitespace immediately before HTTP/.
	pEnd := hi
	if pEnd == 0 || !isASCIISpace(rc[pEnd-1]) {
		return false
	}
	for pEnd > 0 && isASCIISpace(rc[pEnd-1]) {
		pEnd--
	}
	pStart := skipSpaces(rc, mEnd)
	if pStart > pEnd {
		pEnd = pStart
	}
	f.path = rc[pStart:pEnd]
	return true
}

// scanOptionalQuotes parses the optional `\s+"referer"\s+"user_agent"` tail.
func scanOptionalQuotes(s string, i int, f *accessFields) {
	n := skipSpaces(s, i)
	if n == i || n >= len(s) || s[n] != '"' {
		return
	}
	a := strings.IndexByte(s[n+1:], '"')
	if a < 0 {
		return
	}
	ref := s[n+1 : n+1+a]
	m := n + 1 + a + 1
	p := skipSpaces(s, m)
	if p == m || p >= len(s) || s[p] != '"' {
		return
	}
	b := strings.IndexByte(s[p+1:], '"')
	if b < 0 {
		return
	}
	f.referer = ref
	f.userAgent = s[p+1 : p+1+b]
	f.hasReferer = true
}

func init() { Register(nginxAccessParser{}, 80) }

// nginxErrorParser handles Nginx error log lines:
//
//	2024/01/15 10:23:45 [error] 1234#0: *5 open() failed ...
//
// Manual, regex-free scanner equivalent to:
//
//	^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+
//	 \[(debug|info|notice|warn|error|crit|alert|emerg)\]\s+(\d+#\d+):\s*(.*)$
type nginxErrorParser struct{}

var nginxErrLevels = []string{"debug", "info", "notice", "warn", "error", "crit", "alert", "emerg"}

func (nginxErrorParser) Name() string { return "nginx-error" }

func (nginxErrorParser) Detect(line string) bool {
	_, ok := scanNginxErr(trimSpace(line))
	return ok
}

func (nginxErrorParser) Parse(line string, e *Entry) bool {
	f, ok := scanNginxErr(trimSpace(line))
	if !ok {
		return false
	}
	if e.Time.IsZero() {
		if t, ok := parseTime(f.ts); ok {
			e.Time = t
		}
	}
	if e.Level == LevelUnknown {
		e.Level = ParseLevel(f.level)
	}
	e.Fields["pid"] = f.pid
	e.Message = f.message
	return true
}

type nginxErrFields struct {
	ts, level, pid, message string
}

func scanNginxErr(s string) (nginxErrFields, bool) {
	var f nginxErrFields
	if matchDateTimeSlash(s) < 0 {
		return f, false
	}
	f.ts = s[:19]
	i := skipSpaces(s, 19)
	if i == 19 {
		return f, false
	}

	// [level]
	if i >= len(s) || s[i] != '[' {
		return f, false
	}
	i++
	lvlEnd := -1
	for _, kw := range nginxErrLevels {
		if len(s)-i >= len(kw)+1 && s[i:i+len(kw)] == kw && s[i+len(kw)] == ']' {
			f.level = kw
			lvlEnd = i + len(kw) + 1
			break
		}
	}
	if lvlEnd < 0 {
		return f, false
	}
	i = skipSpaces(s, lvlEnd)
	if i == lvlEnd {
		return f, false
	}

	// pid: \d+#\d+
	ds := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i == ds || i >= len(s) || s[i] != '#' {
		return f, false
	}
	i++
	hs := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i == hs {
		return f, false
	}
	f.pid = s[ds:i]

	// ':' then \s* then message
	if i >= len(s) || s[i] != ':' {
		return f, false
	}
	i++
	i = skipSpaces(s, i)
	f.message = s[i:]
	return f, true
}

func init() { Register(nginxErrorParser{}, 75) }
