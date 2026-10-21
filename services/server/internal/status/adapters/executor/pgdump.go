// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Error codes of PgDump besides those of run.
const (
	codePgDumpNotConfigured   = "pg_dump_not_configured"
	codePgDumpPasswordInvalid = "pg_dump_password_invalid" // #nosec G101 -- an error code, not a credential
	codePgDumpFailed          = "pg_dump_failed"
)

// PgDump implements app.Executor: "pg_dump -Fc" of Config.DB into path. The password goes
// through a passfile (PGPASSFILE, mode 0600) in a fresh directory next to path, removed
// afterwards, never through PGPASSWORD, which other processes of the user could read from the
// environment of the child for its whole run.
func (e *Executor) PgDump(ctx context.Context, path string) (int64, error) {
	db := e.cfg.DB
	if db.Password == nil {
		return 0, execError(codePgDumpNotConfigured)
	}
	pw, err := db.Password(ctx)
	if err != nil {
		return 0, execError(codeSecretUnreadable)
	}
	// A line break cannot be written into a passfile line.
	if len(pw) == 0 || strings.ContainsAny(string(pw), "\r\n") {
		return 0, execError(codePgDumpPasswordInvalid)
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".pgpass-")
	if err != nil {
		return 0, execError(codePgDumpFailed)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	passfile := filepath.Join(dir, "pgpass")
	port := strconv.Itoa(db.Port)
	line := strings.Join([]string{passEscape(db.Host), port, passEscape(db.Name), passEscape(db.User), passEscape(string(pw))}, ":") + "\n"
	if err := os.WriteFile(passfile, []byte(line), 0o600); err != nil {
		return 0, execError(codePgDumpFailed)
	}
	args := []string{"-Fc", "--no-password", "--host=" + db.Host, "--port=" + port,
		"--username=" + db.User, "--dbname=" + db.Name, "--file=" + path}
	if _, err := e.run(ctx, toolPgDump, e.cfg.PgDump, args, []string{"PGPASSFILE=" + passfile}, nil); err != nil {
		return 0, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, execError(toolPgDump + "_output_invalid")
	}
	return fi.Size(), nil
}

// passEscape escapes a field of a passfile line: \ and : get a backslash (libpq "The Password
// File").
func passEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(s)
}
