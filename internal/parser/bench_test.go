package parser

import "testing"

// benchLines holds one representative line per supported format. They are used
// by the per-format and dispatcher benchmarks below.
var benchLines = map[string]string{
	"json":              `{"time":"2024-01-15T10:23:45Z","level":"INFO","msg":"slog message","user":"alice","request_id":"abc123"}`,
	"logfmt":            `time="2024-01-15T10:23:45Z" level=info msg="disk almost full" host=srv1 count=42`,
	"java":              `2024-01-15 10:23:45,123 INFO  [main] com.example.App - Application started`,
	"python":            `2024-01-15 10:23:47,500 - myapp.api - ERROR - Failed to fetch user 42`,
	"nginx-access":      `10.0.0.5 - alice [15/Jan/2024:10:23:46 +0000] "POST /api/login HTTP/1.1" 401 87 "https://example.com/" "Mozilla/5.0"`,
	"nginx-error":       `2024/01/15 10:23:45 [error] 1234#0: *5 open() failed`,
	"envoy":             `[2024-01-15T10:23:47.500Z] "GET /health HTTP/1.1" 503 UF 0 0 1 - "-" "kube" "req" "example.com" "10.0.0.3:8080"`,
	"syslog":            `Oct 11 22:14:15 mymachine sshd[1234]: Accepted publickey for user`,
	"generic-timestamp": `2024-01-15 10:23:45 something happened at info level`,
	"plain":             `something went wrong: ERROR reading file`,
}

// benchAll is a mixed slice covering all formats, used for a realistic
// throughput measurement of the dispatcher.
var benchAll = func() []string {
	out := make([]string, 0, len(benchLines))
	for _, v := range benchLines {
		out = append(out, v)
	}
	return out
}()

func BenchmarkParseByFormat(b *testing.B) {
	for name, line := range benchLines {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = Parse(line)
			}
		})
	}
}

func BenchmarkDispatchMixed(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		for _, l := range benchAll {
			_ = Parse(l)
		}
	}
}

func BenchmarkParseLevel(b *testing.B) {
	inputs := []string{"info", "INFO", " Warn ", "warning", "ERROR", "debug", "trace", "fatal", "unknownlevel"}
	b.ReportAllocs()
	for range b.N {
		for _, s := range inputs {
			_ = ParseLevel(s)
		}
	}
}

func BenchmarkParseTime(b *testing.B) {
	inputs := []string{
		"2024-01-15T10:23:45Z",
		"2024-01-15T10:23:45.123456789Z",
		"2024-01-15 10:23:45,123",
		"2024/01/15 10:23:45",
		"15/Jan/2024:10:23:46 +0000",
		"Oct 11 22:14:15",
	}
	b.ReportAllocs()
	for range b.N {
		for _, s := range inputs {
			_, _ = parseTime(s)
		}
	}
}
