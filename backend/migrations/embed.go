// Package migrations embeds the SQL migration files into the server binary
// so deployments that ship only the compiled executable (e.g. Vercel's Go
// web service runtime) can still run migrations at boot. The on-disk layout
// (dialect subdirectories with a legacy flat fallback) is preserved as-is.
package migrations

import "embed"

//go:embed *.sql mysql/*.sql postgres/*.sql sqlite/*.sql
var FS embed.FS
