package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

type Handler struct {
	mu     sync.Mutex
	out    io.Writer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
	color  bool
}

func NewHandler(out io.Writer, level slog.Level) *Handler {
	return &Handler{
		out:   out,
		level: level,
		color: IsTerminal(out),
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var b strings.Builder
	emoji, label, color := LevelStyle(r.Level)
	if h.color {
		fmt.Fprintf(&b, "%s%s%s %s%s%s",
			color,
			emoji,
			label,
			ansiReset,
			r.Message,
			ansiReset,
		)
	} else {
		fmt.Fprintf(&b, "%s%s %s", emoji, label, r.Message)
	}

	allAttrs := append([]slog.Attr{}, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		allAttrs = append(allAttrs, a)
		return true
	})
	AppendAttrs(&b, allAttrs, h.groups, h.color)
	b.WriteByte('\n')

	_, err := io.WriteString(h.out, b.String())
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		out:    h.out,
		level:  h.level,
		color:  h.color,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: append([]string{}, h.groups...),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{
		out:    h.out,
		level:  h.level,
		color:  h.color,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: append(append([]string{}, h.groups...), name),
	}
}

func AppendAttrs(b *strings.Builder, attrs []slog.Attr, groups []string, color bool) {
	for _, a := range attrs {
		a.Value = a.Value.Resolve()
		key := a.Key
		if len(groups) > 0 {
			key = strings.Join(groups, ".") + "." + key
		}
		val := FormatValue(a.Value)
		if color {
			fmt.Fprintf(b, " %s%s%s=%s%s%s",
				ansiDim,
				key,
				ansiReset,
				ansiCyan,
				val,
				ansiReset,
			)
		} else {
			fmt.Fprintf(b, " %s=%s", key, val)
		}
	}
}

func FormatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return fmt.Sprintf("%g", v.Float64())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindAny:
		return fmt.Sprintf("%v", v.Any())
	case slog.KindGroup:
		return FormatGroup(v.Group())
	case slog.KindLogValuer:
		return FormatValue(v.Resolve())
	default:
		return v.String()
	}
}

func FormatGroup(attrs []slog.Attr) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, a := range attrs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%s", a.Key, FormatValue(a.Value))
	}
	b.WriteByte('}')
	return b.String()
}

func IsTerminal(out io.Writer) bool {
	f, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	if v, found := os.LookupEnv("NO_COLOR"); found && v != "" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiCyan   = "\x1b[36m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiGreen  = "\x1b[32m"
	ansiBlue   = "\x1b[34m"
)

func LevelStyle(level slog.Level) (string, string, string) {
	switch {
	case level >= slog.LevelError:
		return "❌", "ERROR", ansiRed
	case level >= slog.LevelWarn:
		return "⚠️", "WARN", ansiYellow
	case level >= slog.LevelInfo:
		return "✅", "INFO", ansiGreen
	default:
		return "🐛", "DEBUG", ansiBlue
	}
}
