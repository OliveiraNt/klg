package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This file pins the behavior of the format parsers by keeping the ORIGINAL
// regexp-based reference implementations here (in the test binary only) and
// asserting that the production (regex-free) implementations produce identical
// results across the full testdata corpus plus crafted edge cases.
//
// It is the safety net for PRIORITY 1 (removing regexp from the parsers): the
// production code may change how it scans, but must not change what it emits.

var (
	refJavaRe       = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}[,.]\d{3})\s+(TRACE|DEBUG|INFO|WARN|ERROR|FATAL)\b\s*(?:\[([^\]]+)\])?\s*([\w.$]+)?\s*[-:]?\s*(.*)$`)
	refPythonRe     = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:[,.]\d{3})?)\s-\s(\S+)\s-\s(DEBUG|INFO|WARNING|WARN|ERROR|CRITICAL|FATAL)\s-\s(.*)$`)
	refAccessRe     = regexp.MustCompile(`^(\S+)\s+\S+\s+(\S+)\s+\[([^\]]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"\s+(\d{3})\s+(\S+)(?:\s+"([^"]*)"\s+"([^"]*)")?`)
	refNginxErrRe   = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+\[(debug|info|notice|warn|error|crit|alert|emerg)\]\s+(\d+#\d+):\s*(.*)$`)
	refEnvoyRe      = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}T[\d:.Z+-]+)\]\s+"([A-Z]+)\s+([^"]*)\s+HTTP/[\d.]+"\s+(\d{3})\s+(\S+)`)
	refSyslog3164Re = regexp.MustCompile(`^(?:<\d+>)?([A-Z][a-z]{2}\s+\d{1,2} \d{2}:\d{2}:\d{2})\s+(\S+)\s+([^:\[]+)(?:\[(\d+)\])?:\s*(.*)$`)
	refSyslog5424Re = regexp.MustCompile(`^<\d+>\d\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:\[[^\]]*\]|-)\s*(.*)$`)
	refGenericTsRe  = regexp.MustCompile(`^(\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\s+(.*)$`)
)

func newEntry() *Entry { return &Entry{Fields: map[string]string{}} }

func refJava(line string) (*Entry, bool) {
	m := refJavaRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		e.Time = t
	}
	e.Level = ParseLevel(m[2])
	if m[3] != "" {
		e.Fields["thread"] = m[3]
	}
	if m[4] != "" {
		e.Fields["logger"] = m[4]
	}
	e.Message = strings.TrimSpace(m[5])
	return e, true
}

func refPython(line string) (*Entry, bool) {
	m := refPythonRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		e.Time = t
	}
	e.Fields["logger"] = m[2]
	e.Level = ParseLevel(m[3])
	e.Message = m[4]
	return e, true
}

func refAccess(line string) (*Entry, bool) {
	m := refAccessRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[3]); ok {
		e.Time = t
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
	if len(m[6]) == 3 {
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
	return e, true
}

func refNginxErr(line string) (*Entry, bool) {
	m := refNginxErrRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		e.Time = t
	}
	e.Level = ParseLevel(m[2])
	e.Fields["pid"] = m[3]
	e.Message = m[4]
	return e, true
}

func refEnvoy(line string) (*Entry, bool) {
	m := refEnvoyRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		e.Time = t
	}
	e.Fields["method"] = m[2]
	e.Fields["path"] = m[3]
	e.Fields["status"] = m[4]
	e.Fields["response_flags"] = m[5]
	if len(m[4]) == 3 {
		switch m[4][0] {
		case '5':
			e.Level = LevelError
		case '4':
			e.Level = LevelWarn
		default:
			e.Level = LevelInfo
		}
	}
	e.Message = m[2] + " " + m[3] + " " + m[4]
	return e, true
}

func refSyslog(line string) (*Entry, bool) {
	s := strings.TrimSpace(line)
	if m := refSyslog5424Re.FindStringSubmatch(s); m != nil {
		e := newEntry()
		if t, ok := parseTime(m[1]); ok {
			e.Time = t
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
		return e, true
	}
	m := refSyslog3164Re.FindStringSubmatch(s)
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		if t.Year() == 0 {
			t = t.AddDate(time.Now().Year(), 0, 0)
		}
		e.Time = t
	}
	e.Fields["host"] = m[2]
	e.Fields["app"] = strings.TrimSpace(m[3])
	if m[4] != "" {
		e.Fields["pid"] = m[4]
	}
	e.Message = m[5]
	return e, true
}

func refGeneric(line string) (*Entry, bool) {
	m := refGenericTsRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil, false
	}
	e := newEntry()
	if t, ok := parseTime(m[1]); ok {
		e.Time = t
	}
	rest := m[2]
	lower := strings.ToLower(rest)
	for _, kw := range []string{"error", "warn", "info", "debug", "trace", "fatal", "panic"} {
		if strings.Contains(lower, kw) {
			e.Level = ParseLevel(kw)
			break
		}
	}
	e.Message = rest
	return e, true
}

// corpus gathers every line of every testdata log plus crafted edge cases.
func corpus(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata")
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	var lines []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := os.Open(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("open %s: %v", f.Name(), err)
		}
		sc := bufio.NewScanner(data)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		data.Close()
	}
	// Crafted edge cases exercising optional groups and boundaries.
	lines = append(lines,
		`2024-01-15 10:23:45.999 DEBUG message without brackets or logger`,
		`2024-01-15 10:23:45,123 WARN [t]  - only thread`,
		`2024-01-15 10:23:45,000 ERROR com.foo.Bar plain logger no bracket`,
		`2024-01-15 10:23:45,000 - a.b - WARN - warn python`,
		`10.0.0.1 - - [15/Jan/2024:10:23:45 +0000] "GET / HTTP/2.0" 200 5`,
		`2001:db8::1 - - [15/Jan/2024:10:23:45 +0000] "GET / HTTP/1.1" 302 0 "-" "UA"`,
		`2024/01/15 10:23:45 [crit] 9#9: crit message`,
		`[2024-01-15T10:23:45Z] "PUT /x HTTP/1.1" 204 -`,
		`Oct  1 09:08:07 host app: short day`,
		`<13>Feb 28 01:02:03 host kernel[9]: with pid`,
		`<165>1 2003-10-11T22:14:15.003Z host app 1 ID47 - msg`,
		`2024-01-15T10:23:45+02:00 free form warn text`,
		`not a log line at all`,
		``,
	)
	return lines
}

type refFn func(string) (*Entry, bool)

func assertParserEquiv(t *testing.T, name string, p Parser, ref refFn, lines []string) {
	t.Helper()
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		refE, refOK := ref(line)
		newE := newEntry()
		newOK := p.Detect(line) && p.Parse(trimmed, newE)
		if refOK != newOK {
			t.Errorf("%s: detect/parse mismatch for %q: ref=%v new=%v", name, line, refOK, newOK)
			continue
		}
		if !refOK {
			continue
		}
		if !refE.Time.Equal(newE.Time) || refE.Level != newE.Level ||
			refE.Message != newE.Message || !reflect.DeepEqual(refE.Fields, newE.Fields) {
			t.Errorf("%s: entry mismatch for %q\n ref=%+v\n new=%+v", name, line, refE, newE)
		}
	}
}

func TestEquivJava(t *testing.T) {
	assertParserEquiv(t, "java", javaParser{}, refJava, corpus(t))
}
func TestEquivPython(t *testing.T) {
	assertParserEquiv(t, "python", pythonParser{}, refPython, corpus(t))
}
func TestEquivNginxAccess(t *testing.T) {
	assertParserEquiv(t, "nginx-access", nginxAccessParser{}, refAccess, corpus(t))
}
func TestEquivNginxError(t *testing.T) {
	assertParserEquiv(t, "nginx-error", nginxErrorParser{}, refNginxErr, corpus(t))
}
func TestEquivEnvoy(t *testing.T) {
	assertParserEquiv(t, "envoy", envoyParser{}, refEnvoy, corpus(t))
}
func TestEquivSyslog(t *testing.T) {
	assertParserEquiv(t, "syslog", syslogParser{}, refSyslog, corpus(t))
}
func TestEquivGeneric(t *testing.T) {
	assertParserEquiv(t, "generic-timestamp", genericTimestampParser{}, refGeneric, corpus(t))
}

var (
	refCodeFrameRe = regexp.MustCompile(`\.[A-Za-z]{1,4}:\d+`)
	refPyErrorRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*Error:`)
)

func TestEquivMultilineHelpers(t *testing.T) {
	lines := append(corpus(t),
		"\tat com.example.Foo.bar(Foo.java:42)",
		"main.go:10 +0x1f",
		"File \"api.py\", line 87",
		"KeyError: 42",
		"ValueError: bad",
		"Error: not matched",
		"error: lowercase",
		"AError:",
		"ErrorX: nope",
		"HttpError: boom",
		"a.toolongext:5",
		"a.go:", // no digit
		"pkg.v2:3",
	)
	for _, line := range lines {
		if got, want := containsCodeFrame(line), refCodeFrameRe.MatchString(line); got != want {
			t.Errorf("containsCodeFrame(%q)=%v want %v", line, got, want)
		}
		if got, want := matchPyError(line), refPyErrorRe.MatchString(line); got != want {
			t.Errorf("matchPyError(%q)=%v want %v", line, got, want)
		}
	}
}
