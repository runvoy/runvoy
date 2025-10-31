package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

const (
	// ANSI color codes
	colorReset    = "\033[0m"
	colorRed      = "\033[31m"
	colorGreen    = "\033[32m"
	colorYellow   = "\033[33m"
	colorBlue     = "\033[34m"
	colorMagenta  = "\033[35m"
	colorCyan     = "\033[36m"
	colorGray     = "\033[90m"
	colorWhite    = "\033[97m"
	colorBoldRed  = "\033[1;31m"
	colorBoldCyan = "\033[1;36m"
)

// colorHandler is a slog.Handler that formats log records with ANSI colors
type colorHandler struct {
	opts            *slog.HandlerOptions
	writer          io.Writer
	preformattedAttrs []byte
	groups          []string
}

// NewColorHandler creates a new color handler that formats logs with colors
func NewColorHandler(w io.Writer, opts *slog.HandlerOptions) *colorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &colorHandler{
		opts:   opts,
		writer: w,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle processes the log record and formats it with colors
func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	var buf strings.Builder

	// Format time
	if !r.Time.IsZero() {
		timeStr := r.Time.Format(time.TimeOnly)
		buf.WriteString(colorGray)
		buf.WriteString(timeStr)
		buf.WriteString(colorReset)
		buf.WriteByte(' ')
	}

	// Format level with color
	levelColor := h.levelColor(r.Level)
	levelStr := r.Level.String()
	buf.WriteString(levelColor)
	buf.WriteString(strings.ToUpper(levelStr))
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	// Format message
	buf.WriteString(h.messageColor(r.Level))
	buf.WriteString(r.Message)
	buf.WriteString(colorReset)

	// Add preformatted attrs if any (from WithAttrs)
	if len(h.preformattedAttrs) > 0 {
		buf.WriteByte(' ')
		buf.Write(h.preformattedAttrs)
	}

	// Format attributes from the record
	if r.NumAttrs() > 0 {
		needSpace := len(h.preformattedAttrs) > 0
		r.Attrs(func(a slog.Attr) bool {
			if needSpace {
				buf.WriteByte(' ')
			}
			h.appendAttr(&buf, a)
			needSpace = true
			return true
		})
	}

	buf.WriteByte('\n')

	_, err := fmt.Fprint(h.writer, buf.String())
	return err
}

// levelColor returns the ANSI color code for a log level
func (h *colorHandler) levelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return colorGray
	case slog.LevelInfo:
		return colorCyan
	case slog.LevelWarn:
		return colorYellow
	case slog.LevelError:
		return colorRed
	default:
		return colorWhite
	}
}

// messageColor returns the ANSI color code for the message based on level
func (h *colorHandler) messageColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return colorGray
	case slog.LevelInfo:
		return colorWhite
	case slog.LevelWarn:
		return colorYellow
	case slog.LevelError:
		return colorBoldRed
	default:
		return colorWhite
	}
}

// appendAttr formats and appends an attribute to the buffer
func (h *colorHandler) appendAttr(buf *strings.Builder, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}

	a = h.resolve(a)
	if a.Equal(slog.Attr{}) {
		return
	}

	// Colorize special keys
	key := a.Key
	if len(h.groups) > 0 {
		key = strings.Join(h.groups, ".") + "." + key
	}

	// Colorize key
	buf.WriteString(colorCyan)
	buf.WriteString(key)
	buf.WriteString(colorReset)
	buf.WriteByte('=')

	// Colorize value based on key type
	h.appendValue(buf, a.Value, key)
}

// appendValue formats a Value and appends it to the buffer
func (h *colorHandler) appendValue(buf *strings.Builder, v slog.Value, key string) {
	switch v.Kind() {
	case slog.KindString:
		// Special coloring for error and status fields
		val := v.String()
		if key == "error" {
			buf.WriteString(colorRed)
			buf.WriteString(val)
			buf.WriteString(colorReset)
		} else if key == "status" {
			statusColor := colorCyan
			if strings.Contains(val, "SUCCEEDED") {
				statusColor = colorGreen
			} else if strings.Contains(val, "FAILED") {
				statusColor = colorRed
			}
			buf.WriteString(statusColor)
			buf.WriteString(val)
			buf.WriteString(colorReset)
		} else {
			buf.WriteString(val)
		}
	case slog.KindInt64:
		buf.WriteString(colorYellow)
		fmt.Fprintf(buf, "%d", v.Int64())
		buf.WriteString(colorReset)
	case slog.KindUint64:
		buf.WriteString(colorYellow)
		fmt.Fprintf(buf, "%d", v.Uint64())
		buf.WriteString(colorReset)
	case slog.KindFloat64:
		buf.WriteString(colorYellow)
		fmt.Fprintf(buf, "%g", v.Float64())
		buf.WriteString(colorReset)
	case slog.KindBool:
		buf.WriteString(colorMagenta)
		fmt.Fprintf(buf, "%t", v.Bool())
		buf.WriteString(colorReset)
	case slog.KindDuration:
		buf.WriteString(colorBlue)
		fmt.Fprintf(buf, "%s", v.Duration())
		buf.WriteString(colorReset)
	case slog.KindTime:
		buf.WriteString(colorGray)
		fmt.Fprintf(buf, "%s", v.Time().Format(time.RFC3339))
		buf.WriteString(colorReset)
	case slog.KindGroup:
		attrs := v.Group()
		if len(attrs) == 0 {
			buf.WriteString("{}")
			return
		}
		buf.WriteByte('{')
		for i, a := range attrs {
			if i > 0 {
				buf.WriteByte(' ')
			}
			h.appendAttr(buf, a)
		}
		buf.WriteByte('}')
	case slog.KindAny:
		buf.WriteString(colorWhite)
		fmt.Fprintf(buf, "%v", v.Any())
		buf.WriteString(colorReset)
	default:
		fmt.Fprintf(buf, "%v", v.Any())
	}
}

// resolve resolves an attribute's value
func (h *colorHandler) resolve(a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindLogValuer {
		return a
	}
	v := a.Value.LogValuer().LogValue()
	return slog.Attr{Key: a.Key, Value: v}
}

// WithAttrs returns a new handler with the given attributes
func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	var preformatted strings.Builder
	for i, a := range attrs {
		if i > 0 {
			preformatted.WriteByte(' ')
		}
		h2.appendAttr(&preformatted, a)
	}
	h2.preformattedAttrs = []byte(preformatted.String())
	return &h2
}

// WithGroup returns a new handler with the given group
func (h *colorHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	// Copy groups slice to avoid sharing underlying array
	h2.groups = make([]string, len(h.groups), len(h.groups)+1)
	copy(h2.groups, h.groups)
	h2.groups = append(h2.groups, name)
	return &h2
}
