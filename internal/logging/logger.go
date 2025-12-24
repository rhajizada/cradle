package logging

import (
	"io"
	"log/slog"
)

func New(out io.Writer) *slog.Logger {
	return slog.New(NewHandler(out, slog.LevelInfo))
}
