// Package internal holds the logger backend adapters (zap / slog) and
// the shared OutputFormat / level vocabulary the public logger package
// translates user input into.
package internal

type OutputFormat string

const (
	JSONFormat OutputFormat = "json"
	TextFormat OutputFormat = "text"
)
