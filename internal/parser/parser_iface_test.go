package parser

import "testing"

func TestParseJSONDispatch(t *testing.T) {
	e := Parse(`{"time":"2023-10-27T10:00:00Z","level":"info","msg":"hello"}`)
	if e.Message != "hello" {
		t.Errorf("expected message 'hello', got %q", e.Message)
	}
	if e.Level != LevelInfo {
		t.Errorf("expected LevelInfo, got %v", e.Level)
	}
}

func TestParseLogfmtDispatch(t *testing.T) {
	e := Parse(`level=warn msg="disk almost full" host=srv1`)
	if e.Level != LevelWarn {
		t.Errorf("expected LevelWarn, got %v", e.Level)
	}
	if e.Message != "disk almost full" {
		t.Errorf("unexpected message %q", e.Message)
	}
	if e.Fields["host"] != "srv1" {
		t.Errorf("expected host=srv1, got %q", e.Fields["host"])
	}
}

func TestParsePlainFallback(t *testing.T) {
	e := Parse(`something went wrong: ERROR reading file`)
	if e.Level != LevelError {
		t.Errorf("expected LevelError, got %v", e.Level)
	}
}

func TestRegistryPriorityOrder(t *testing.T) {
	ps := Parsers()
	if len(ps) < 3 {
		t.Fatalf("expected at least 3 parsers, got %d", len(ps))
	}
	if ps[0].Name() != "json" {
		t.Errorf("expected json first, got %s", ps[0].Name())
	}
	if ps[len(ps)-1].Name() != "plain" {
		t.Errorf("expected plain last, got %s", ps[len(ps)-1].Name())
	}
}

type fakeParser struct{ called *bool }

func (fakeParser) Name() string         { return "fake" }
func (f fakeParser) Detect(string) bool { *f.called = true; return true }
func (fakeParser) Parse(_ string, e *Entry) bool {
	e.Message = "from-fake"
	e.Level = LevelDebug
	return true
}

func TestRegisterExtensibility(t *testing.T) {
	called := false
	// Save/restore registry so this test doesn't affect the others.
	saved := registry
	defer func() { registry = saved }()

	Register(fakeParser{called: &called}, 1000)
	e := Parse(`{"level":"info","msg":"real"}`)
	if !called {
		t.Fatal("fake parser Detect not invoked")
	}
	if e.Message != "from-fake" {
		t.Errorf("expected fake parser to win, got message %q", e.Message)
	}
}
