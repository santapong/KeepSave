// Package version exposes the application's semantic version as a single
// constant so health endpoints, structured logs, and metrics labels all
// agree on the string. Update on each release.
package version

// Version is the semantic version surfaced by /healthz, /readyz, and the
// startup log line.
const Version = "1.1.0"
