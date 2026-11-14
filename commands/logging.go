package commands

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Supported values for the --log-format flag.
const (
	logFormatText = "text"
	logFormatJSON = "json"
)

// newLogger returns a slog.Logger that writes to w in the given format.
func newLogger(w io.Writer, format string) (*slog.Logger, error) {
	switch strings.ToLower(format) {
	case logFormatText:
		return slog.New(slog.NewTextHandler(w, nil)), nil
	case logFormatJSON:
		return slog.New(slog.NewJSONHandler(w, nil)), nil
	default:
		return nil, fmt.Errorf("invalid log format %q: must be %q or %q", format, logFormatText, logFormatJSON)
	}
}
