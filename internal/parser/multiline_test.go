package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsContinuation(t *testing.T) {
	prev := &Entry{Message: "prev"}
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"leading tab", "\tat com.Foo.bar(Foo.java:1)", true},
		{"leading space", "  File \"a.py\", line 1", true},
		{"at prefix", "at com.Foo.bar(Foo.java:1)", true},
		{"caused by", "Caused by: java.io.IOException", true},
		{"dots more", "... 3 more", true},
		{"goroutine", "goroutine 1 [running]:", true},
		{"panic", "panic: nil pointer", true},
		{"free exception text", "java.lang.RuntimeException: boom", true},
		{"json new event", `{"level":"info","msg":"hi"}`, false},
		{"logfmt new event", `time="2024-01-15T10:23:45Z" level=info msg=hi`, false},
		{"java new event", `2024-01-15 10:23:45,123 INFO  [main] com.example.App - hi`, false},
		{"rfc3339 leading", `2024-01-15T10:23:45Z hello world`, false},
		{"empty line", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isContinuation(c.line, prev); got != c.want {
				t.Fatalf("isContinuation(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
	if isContinuation("\tat X", nil) {
		t.Fatal("nil prev must not be a continuation")
	}
}

func feedAll(agg *LineAggregator, lines []string) []Entry {
	var out []Entry
	for _, l := range lines {
		if l == "" {
			continue
		}
		if e, ready := agg.Feed(l); ready {
			out = append(out, e)
		}
	}
	if e, ready := agg.Flush(); ready {
		out = append(out, e)
	}
	return out
}

func readTestdata(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func TestAggregatorJavaStackTrace(t *testing.T) {
	agg := NewLineAggregator(true)
	entries := feedAll(agg, readTestdata(t, "java-stacktrace.log"))
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	first := entries[0]
	if first.Level != LevelError {
		t.Errorf("first level: want ERROR, got %v", first.Level)
	}
	if !strings.Contains(first.Message, "Caused by:") || !strings.Contains(first.Message, "\tat com.example") {
		t.Errorf("first message missing stack frames:\n%s", first.Message)
	}
	if !strings.HasPrefix(first.Message, "Failed to process batch") {
		t.Errorf("first message should start with header, got %q", first.Message)
	}
}

func TestAggregatorGoPanic(t *testing.T) {
	agg := NewLineAggregator(true)
	entries := feedAll(agg, readTestdata(t, "go-panic.log"))
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Level != LevelFatal {
		t.Errorf("level: want FATAL, got %v", e.Level)
	}
	if !strings.Contains(e.Message, "goroutine 1 [running]:") {
		t.Errorf("message missing goroutine line:\n%s", e.Message)
	}
	if !strings.Contains(e.Message, "/app/main.go:42") {
		t.Errorf("message missing frame:\n%s", e.Message)
	}
}

func TestAggregatorPythonTraceback(t *testing.T) {
	agg := NewLineAggregator(true)
	entries := feedAll(agg, readTestdata(t, "python-traceback.log"))
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	first := entries[0]
	if first.Level != LevelError {
		t.Errorf("first level: want ERROR, got %v", first.Level)
	}
	if !strings.Contains(first.Message, "Traceback (most recent call last):") {
		t.Errorf("traceback header missing:\n%s", first.Message)
	}
	if !strings.Contains(first.Message, "KeyError: 42") {
		t.Errorf("final exception line missing:\n%s", first.Message)
	}
}

func TestAggregatorMixedStream(t *testing.T) {
	agg := NewLineAggregator(true)
	lines := []string{
		`{"level":"info","msg":"first json"}`,
		`some plain message without timestamp`,
		`  continuation of plain`,
		`{"level":"warn","msg":"second json"}`,
	}
	entries := feedAll(agg, lines)
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	if entries[0].Message != "first json" {
		t.Errorf("entry0: %q", entries[0].Message)
	}
	if !strings.Contains(entries[1].Message, "continuation of plain") {
		t.Errorf("entry1 should include continuation, got %q", entries[1].Message)
	}
	if entries[2].Message != "second json" {
		t.Errorf("entry2: %q", entries[2].Message)
	}
}

func TestAggregatorDisabled(t *testing.T) {
	agg := NewLineAggregator(false)
	lines := readTestdata(t, "go-panic.log")
	entries := feedAll(agg, lines)
	// With aggregation disabled, every non-empty physical line yields an entry.
	nonEmpty := 0
	for _, l := range lines {
		if l != "" {
			nonEmpty++
		}
	}
	if len(entries) != nonEmpty {
		t.Fatalf("disabled: want %d entries, got %d", nonEmpty, len(entries))
	}
}
