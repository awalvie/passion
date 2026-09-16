package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-isatty"
)

// prettyHandler prints one readable line per record, with colour. slog's own text handler
// puts time= and level= in front of the message, which buries the message itself.
//
//	11:54:45 WARN  catalog file ignored  tree=shipped file=movements/x.yaml
type prettyHandler struct {
	mu    *sync.Mutex
	out   io.Writer
	level slog.Level
	color bool
	attrs []slog.Attr
	group string
}

const (
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	reset  = "\033[0m"
)

func (h *prettyHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	b.WriteString(h.paint(dim, r.Time.Format("15:04:05")))
	b.WriteByte(' ')
	b.WriteString(h.paint(levelColor(r.Level), fmt.Sprintf("%-5s", r.Level.String())))
	b.WriteByte(' ')
	b.WriteString(r.Message)

	write := func(a slog.Attr) {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		b.WriteString("  " + h.paint(dim, key+"="))
		b.WriteString(fmt.Sprint(a.Value.Any()))
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		write(a)
		return true
	})
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	next := *h
	if h.group != "" {
		name = h.group + "." + name
	}
	next.group = name
	return &next
}

func (h *prettyHandler) paint(color, s string) string {
	if !h.color {
		return s
	}
	return color + s + reset
}

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return red
	case l >= slog.LevelWarn:
		return yellow
	case l >= slog.LevelInfo:
		return cyan
	default:
		return green
	}
}

func newPrettyHandler(w io.Writer, level slog.Level) slog.Handler {
	color := false
	if f, ok := w.(*os.File); ok {
		color = isatty.IsTerminal(f.Fd()) && os.Getenv("NO_COLOR") == ""
	}
	return &prettyHandler{mu: &sync.Mutex{}, out: w, level: level, color: color}
}
