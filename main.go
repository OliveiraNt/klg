package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/OliveiraNt/klg/internal/formatter"
	"github.com/OliveiraNt/klg/internal/parser"
)

// Injected at build time via -ldflags (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	noColor     bool
	showRaw     bool
	minLevel    string
	timeFormat  string
	jsonPretty  bool
	showVersion bool
)

func main() {
	flag.BoolVar(&noColor, "no-color", false, "disable colored output")
	flag.BoolVar(&showRaw, "raw", false, "append the original line after the formatted output")
	flag.StringVar(&minLevel, "level", "", "filter by minimum level (debug|info|warn|error)")
	flag.StringVar(&timeFormat, "time", "15:04:05", "time layout (Go time format)")
	flag.BoolVar(&jsonPretty, "json-pretty", false, "render JSON values (objects/arrays) with indentation and colors")
	flag.BoolVar(&showVersion, "version", false, "print version information and exit")
	flag.Usage = usage
	flag.Parse()

	if showVersion || (flag.NArg() == 1 && flag.Arg(0) == "version") {
		fmt.Printf("klg %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	if isTerminal(os.Stdin) && flag.NArg() == 0 {
		usage()
		os.Exit(2)
	}

	f := formatter.New(formatter.Options{
		NoColor:    noColor || !isTerminal(os.Stdout),
		ShowRaw:    showRaw,
		MinLevel:   parser.ParseLevel(minLevel),
		TimeFormat: timeFormat,
		JSONPretty: jsonPretty,
	})

	if err := run(os.Stdin, os.Stdout, f); err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "klg:", err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer, f *formatter.Formatter) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		entry := parser.Parse(line)
		if !f.Accept(entry) {
			continue
		}
		fmt.Fprintln(out, f.Format(entry))
	}
	return scanner.Err()
}

func usage() {
	fmt.Fprintf(os.Stderr, `klg - kubectl log formatter

Usage:
  kubectl logs <pod> [-f] | klg [flags]
  klg version

Flags:
`)
	flag.PrintDefaults()
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
