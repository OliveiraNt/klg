// Package formatter renderiza entradas de log normalizadas em uma única linha
// legível (ou em bloco, para JSON) usando lipgloss para estilização.
package formatter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/OliveiraNt/klg/internal/parser"
)

// Options controla o comportamento do Formatter.
type Options struct {
	NoColor    bool
	ShowRaw    bool
	MinLevel   parser.Level
	TimeFormat string
	// JSONPretty ativa a renderização de qualquer campo cujo valor seja um
	// objeto/array JSON válido em bloco indentado e colorido.
	JSONPretty bool
}

// Formatter é seguro para uso em uma única goroutine (uso típico da CLI).
type Formatter struct {
	opts Options

	styleTime    lipgloss.Style
	styleMsg     lipgloss.Style
	styleKey     lipgloss.Style
	styleEq      lipgloss.Style
	styleVal     lipgloss.Style
	styleRaw     lipgloss.Style
	styleWarnMsg lipgloss.Style
	styleErrMsg  lipgloss.Style

	levelStyles map[parser.Level]lipgloss.Style

	// estilos para JSON pretty
	jsonKey    lipgloss.Style
	jsonString lipgloss.Style
	jsonNumber lipgloss.Style
	jsonBool   lipgloss.Style
	jsonNull   lipgloss.Style
	jsonPunc   lipgloss.Style
}

// New cria um Formatter com as opções fornecidas.
func New(opts Options) *Formatter {
	if opts.TimeFormat == "" {
		opts.TimeFormat = "15:04:05"
	}
	f := &Formatter{opts: opts}
	f.initStyles()
	return f
}

func (f *Formatter) initStyles() {
	// Desliga cores globalmente se solicitado.
	if f.opts.NoColor {
		lipgloss.SetColorProfile(0) // Ascii
	}

	f.styleTime = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	f.styleMsg = lipgloss.NewStyle()
	f.styleKey = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	f.styleEq = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	f.styleVal = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	f.styleRaw = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	f.styleWarnMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	f.styleErrMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// Nível: badge com padding e cor de fundo por severidade.
	base := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	f.levelStyles = map[parser.Level]lipgloss.Style{
		parser.LevelTrace:   base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("240")),
		parser.LevelDebug:   base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")),
		parser.LevelInfo:    base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("34")),
		parser.LevelWarn:    base.Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214")),
		parser.LevelError:   base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("196")),
		parser.LevelFatal:   base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("124")),
		parser.LevelUnknown: base.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")),
	}

	f.jsonKey = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	f.jsonString = lipgloss.NewStyle().Foreground(lipgloss.Color("113"))
	f.jsonNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	f.jsonBool = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	f.jsonNull = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	f.jsonPunc = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
}

// Accept determina se a entrada passa no filtro de nível.
func (f *Formatter) Accept(e parser.Entry) bool {
	if f.opts.MinLevel == parser.LevelUnknown {
		return true
	}
	if e.Level == parser.LevelUnknown {
		return true
	}
	return e.Level >= f.opts.MinLevel
}

// Format devolve a representação renderizada de uma entrada de log.
func (f *Formatter) Format(e parser.Entry) string {
	var b strings.Builder

	// Timestamp
	if !e.Time.IsZero() {
		b.WriteString(f.styleTime.Render(e.Time.Local().Format(f.opts.TimeFormat)))
		b.WriteByte(' ')
	}

	// Level badge
	lvlStyle, ok := f.levelStyles[e.Level]
	if !ok {
		lvlStyle = f.levelStyles[parser.LevelUnknown]
	}
	b.WriteString(lvlStyle.Render(e.Level.String()))
	b.WriteByte(' ')

	// Message
	msg := strings.TrimSpace(e.Message)
	switch e.Level {
	case parser.LevelError, parser.LevelFatal:
		b.WriteString(f.styleErrMsg.Render(msg))
	case parser.LevelWarn:
		b.WriteString(f.styleWarnMsg.Render(msg))
	default:
		b.WriteString(f.styleMsg.Render(msg))
	}

	// Fields ordenados
	if len(e.Fields) > 0 {
		keys := make([]string, 0, len(e.Fields))
		for k := range e.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var jsonBlocks []string
		for _, k := range keys {
			v := e.Fields[k]
			if f.opts.JSONPretty && isJSONObjectOrArray(v) {
				jsonBlocks = append(jsonBlocks, f.renderJSONField(k, v))
				continue
			}
			b.WriteByte(' ')
			b.WriteString(f.styleKey.Render(k))
			b.WriteString(f.styleEq.Render("="))
			b.WriteString(f.styleVal.Render(quoteIfNeeded(v)))
		}
		for _, blk := range jsonBlocks {
			b.WriteByte('\n')
			b.WriteString(blk)
		}
	}

	if f.opts.ShowRaw {
		b.WriteByte('\n')
		b.WriteString(f.styleRaw.Render("  raw: " + e.Raw))
	}

	return b.String()
}

func isJSONObjectOrArray(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}
	if !(s[0] == '{' || s[0] == '[') {
		return false
	}
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}

func (f *Formatter) renderJSONField(key, raw string) string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "  " + f.styleKey.Render(key) + f.styleEq.Render("=") + f.styleVal.Render(raw)
	}
	pretty := f.colorizeJSON(v, "  ")
	header := f.styleKey.Render(key) + f.styleEq.Render(":")
	return "  " + header + "\n" + pretty
}

// colorizeJSON renderiza recursivamente um valor JSON já decodificado com
// indentação e cores via lipgloss.
func (f *Formatter) colorizeJSON(v any, indent string) string {
	const step = "  "
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return indent + f.jsonPunc.Render("{}")
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString(indent + f.jsonPunc.Render("{") + "\n")
		for i, k := range keys {
			b.WriteString(indent + step)
			b.WriteString(f.jsonKey.Render(fmt.Sprintf("%q", k)))
			b.WriteString(f.jsonPunc.Render(": "))
			inner := f.colorizeJSON(x[k], indent+step)
			b.WriteString(strings.TrimPrefix(inner, indent+step))
			if i < len(keys)-1 {
				b.WriteString(f.jsonPunc.Render(","))
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + f.jsonPunc.Render("}"))
		return b.String()
	case []any:
		if len(x) == 0 {
			return indent + f.jsonPunc.Render("[]")
		}
		var b strings.Builder
		b.WriteString(indent + f.jsonPunc.Render("[") + "\n")
		for i, item := range x {
			inner := f.colorizeJSON(item, indent+step)
			b.WriteString(inner)
			if i < len(x)-1 {
				b.WriteString(f.jsonPunc.Render(","))
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + f.jsonPunc.Render("]"))
		return b.String()
	case string:
		return indent + f.jsonString.Render(fmt.Sprintf("%q", x))
	case bool:
		return indent + f.jsonBool.Render(fmt.Sprintf("%t", x))
	case nil:
		return indent + f.jsonNull.Render("null")
	case float64:
		// json.Unmarshal usa float64 para números.
		return indent + f.jsonNumber.Render(trimFloat(x))
	default:
		b, _ := json.Marshal(x)
		return indent + f.jsonString.Render(string(b))
	}
}

func trimFloat(x float64) string {
	if x == float64(int64(x)) {
		return fmt.Sprintf("%d", int64(x))
	}
	return fmt.Sprintf("%g", x)
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
