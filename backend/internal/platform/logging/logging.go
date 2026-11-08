// Package logging builds the process logger: log/slog with a JSON handler.
//
// Logs never contain amounts, vendor names or notes.
package logging

import (
	"io"
	"log/slog"
)

// New returns a JSON logger that writes to w at level, one of debug, info, warn or error.
// An unknown level logs at info.
func New(w io.Writer, level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: l}))
}
