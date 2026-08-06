package parser

import "testing"

// dispatchedParser returns the name of the first registered parser whose
// Detect matches, mirroring what dispatch does internally.
func dispatchedParser(line string) string {
	for _, p := range Parsers() {
		if p.Detect(line) {
			return p.Name()
		}
	}
	return ""
}

func TestJavaFormat(t *testing.T) {
	line := `2024-01-15 10:23:45,123 INFO  [main] com.example.App - Application started`
	if got := dispatchedParser(line); got != "java" {
		t.Fatalf("expected java parser, got %q", got)
	}
	e := Parse(line)
	if e.Level != LevelInfo {
		t.Errorf("level: want INFO, got %v", e.Level)
	}
	if e.Message != "Application started" {
		t.Errorf("message: got %q", e.Message)
	}
	if e.Fields["thread"] != "main" {
		t.Errorf("thread: got %q", e.Fields["thread"])
	}
	if e.Fields["logger"] != "com.example.App" {
		t.Errorf("logger: got %q", e.Fields["logger"])
	}
	if e.Time.IsZero() {
		t.Error("expected time to be parsed")
	}
}

func TestPythonFormat(t *testing.T) {
	line := `2024-01-15 10:23:47,500 - myapp.api - ERROR - Failed to fetch user 42`
	if got := dispatchedParser(line); got != "python" {
		t.Fatalf("expected python parser, got %q", got)
	}
	e := Parse(line)
	if e.Level != LevelError {
		t.Errorf("level: want ERROR, got %v", e.Level)
	}
	if e.Message != "Failed to fetch user 42" {
		t.Errorf("message: got %q", e.Message)
	}
	if e.Fields["logger"] != "myapp.api" {
		t.Errorf("logger: got %q", e.Fields["logger"])
	}
}

func TestNginxAccessFormat(t *testing.T) {
	line := `10.0.0.5 - alice [15/Jan/2024:10:23:46 +0000] "POST /api/login HTTP/1.1" 401 87 "https://example.com/" "Mozilla/5.0"`
	if got := dispatchedParser(line); got != "nginx-access" {
		t.Fatalf("expected nginx-access parser, got %q", got)
	}
	e := Parse(line)
	if e.Level != LevelWarn {
		t.Errorf("level: want WARN (from 4xx), got %v", e.Level)
	}
	if e.Fields["method"] != "POST" || e.Fields["path"] != "/api/login" || e.Fields["status"] != "401" {
		t.Errorf("access fields wrong: %+v", e.Fields)
	}
	if e.Fields["user"] != "alice" {
		t.Errorf("user: got %q", e.Fields["user"])
	}
}

func TestNginxErrorFormat(t *testing.T) {
	line := `2024/01/15 10:23:45 [error] 1234#0: *5 open() failed`
	if got := dispatchedParser(line); got != "nginx-error" {
		t.Fatalf("expected nginx-error parser, got %q", got)
	}
	e := Parse(line)
	if e.Level != LevelError {
		t.Errorf("level: want ERROR, got %v", e.Level)
	}
	if e.Message != "*5 open() failed" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestEnvoyFormat(t *testing.T) {
	line := `[2024-01-15T10:23:47.500Z] "GET /health HTTP/1.1" 503 UF 0 0 1 - "-" "kube" "req" "example.com" "10.0.0.3:8080"`
	if got := dispatchedParser(line); got != "envoy" {
		t.Fatalf("expected envoy parser, got %q", got)
	}
	e := Parse(line)
	if e.Level != LevelError {
		t.Errorf("level: want ERROR (from 5xx), got %v", e.Level)
	}
	if e.Fields["status"] != "503" {
		t.Errorf("status: got %q", e.Fields["status"])
	}
	if e.Fields["response_flags"] != "UF" {
		t.Errorf("response_flags: got %q", e.Fields["response_flags"])
	}
}

func TestSyslogRFC3164(t *testing.T) {
	line := `Oct 11 22:14:15 mymachine sshd[1234]: Accepted publickey for user`
	if got := dispatchedParser(line); got != "syslog" {
		t.Fatalf("expected syslog parser, got %q", got)
	}
	e := Parse(line)
	if e.Fields["host"] != "mymachine" {
		t.Errorf("host: got %q", e.Fields["host"])
	}
	if e.Fields["app"] != "sshd" {
		t.Errorf("app: got %q", e.Fields["app"])
	}
	if e.Fields["pid"] != "1234" {
		t.Errorf("pid: got %q", e.Fields["pid"])
	}
	if e.Message != "Accepted publickey for user" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestSyslogWithPriority(t *testing.T) {
	line := `<34>Oct 11 22:14:17 mymachine su: 'su root' failed for lonvick`
	if got := dispatchedParser(line); got != "syslog" {
		t.Fatalf("expected syslog parser, got %q", got)
	}
}

func TestGenericTimestampFallback(t *testing.T) {
	line := `2024-01-15T10:23:45Z some free-form message with error inside`
	// splitLeadingTimestamp strips the RFC3339 leading token before dispatch,
	// so the remainder is dispatched. Verify Entry still captures level via
	// the plain fallback and time via splitLeadingTimestamp.
	e := Parse(line)
	if e.Time.IsZero() {
		t.Error("expected leading timestamp to be captured")
	}
	if e.Level != LevelError {
		t.Errorf("level: want ERROR from keyword scan, got %v", e.Level)
	}
}

func TestGenericTimestampDirect(t *testing.T) {
	// A non-RFC3339 leading timestamp (space instead of T) is NOT stripped by
	// splitLeadingTimestamp, so the generic-timestamp parser must handle it.
	line := `2024-01-15 10:23:45 something happened at info level`
	if got := dispatchedParser(line); got != "generic-timestamp" {
		t.Fatalf("expected generic-timestamp parser, got %q", got)
	}
	e := Parse(line)
	if e.Time.IsZero() {
		t.Error("expected time to be parsed")
	}
	if e.Level != LevelInfo {
		t.Errorf("level: want INFO, got %v", e.Level)
	}
}

func TestStructuredLoggersStillJSONOrLogfmt(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{`{"time":"2024-01-15T10:23:45Z","level":"INFO","msg":"slog","user":"alice"}`, "json"},
		{`{"level":"warn","time":"2024-01-15T10:23:45Z","message":"zerolog","request_id":"abc"}`, "json"},
		{`time="2024-01-15T10:23:45Z" level=info msg="slog text" user=alice`, "logfmt"},
	}
	for _, c := range cases {
		if got := dispatchedParser(c.line); got != c.want {
			t.Errorf("line %q: want %s, got %s", c.line, c.want, got)
		}
	}
}
