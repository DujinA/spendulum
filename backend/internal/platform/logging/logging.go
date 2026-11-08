package logging

import (
	"io"
	"log/slog"
)

func New(w io.Writer, level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: l}))
}
