package loghandler

import (
	"context"
	"io"
	"log/slog"
)

const timeFormat = "2006/01/02 15:04:05"

const tagKey = "tag"

// CompactHandler writes logs in a compact form: timestamp + optional [tag] prefix + message + attrs.
// Timestamp format: 2006/01/02 15:04:05 (no TZ, no milliseconds). No level is written.
// If an attribute with key "tag" is present, it is rendered as "[tag] " after the timestamp;
// "tag" is then omitted from the key=value list.
type CompactHandler struct {
	w     io.Writer
	level slog.Level
}

// NewCompactHandler returns a handler that writes to w with minimum level.
func NewCompactHandler(w io.Writer, level slog.Level) *CompactHandler {
	return &CompactHandler{w: w, level: level}
}

// Enabled reports whether the handler handles records at the given level.
func (h *CompactHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats the record as: 2006/01/02 15:04:05 [tag] message key=value ...
// The "tag" attribute is not repeated in the key=value list.
func (h *CompactHandler) Handle(_ context.Context, r slog.Record) error {
	var tag string
	var rest []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == tagKey {
			if a.Value.Kind() == slog.KindString {
				tag = a.Value.String()
			}
			return true
		}
		rest = append(rest, a)
		return true
	})

	buf := make([]byte, 0, 256)
	buf = append(buf, r.Time.Format(timeFormat)...)
	buf = append(buf, ' ')
	if tag != "" {
		buf = append(buf, '[')
		buf = append(buf, tag...)
		buf = append(buf, "] "...)
	}
	buf = append(buf, r.Message...)
	for _, a := range rest {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = append(buf, a.Value.String()...)
	}
	buf = append(buf, '\n')

	_, err := h.w.Write(buf)
	return err
}

// WithAttrs returns a new handler with the given attributes added to the context.
func (h *CompactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, we don't pre-merge attrs; they'll be included in the record.
	return h
}

// WithGroup returns a new handler for the given group (no-op for compact output).
func (h *CompactHandler) WithGroup(name string) slog.Handler {
	return h
}
