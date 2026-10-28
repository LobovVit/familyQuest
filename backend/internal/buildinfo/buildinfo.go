// Package buildinfo contains immutable build metadata injected by the compiler.
// Package buildinfo содержит неизменяемые метаданные, заданные при сборке.
package buildinfo

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)
