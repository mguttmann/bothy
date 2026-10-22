// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// codePgBackRestRequestInvalid reports a request the executor refuses before running pgbackrest.
const codePgBackRestRequestInvalid = "pgbackrest_request_invalid"

// labelPattern is a pgBackRest backup label: full, or full plus differential or incremental.
var labelPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}F(_[0-9]{8}-[0-9]{6}[DI])?$`)

// PgBackRestEnv returns the PGBACKREST_* environment of the children: repo2 on the off-site
// target with the keys of <secrets.dir>/offsite/ (brief M7-02, off-site design), and always
// PGBACKREST_EXPIRE_AUTO=n, so pgBackRest never expires on its own and repo2 is expired only by
// the owner's explicit command. Without an off-site target it is only PGBACKREST_EXPIRE_AUTO=n.
// The lead gives the same environment to the long-running pgbackrest server child. The error
// is an *app.ExecError (secret_unreadable).
func (e *Executor) PgBackRestEnv(ctx context.Context) ([]string, error) {
	_ = ctx
	if e.cfg.Offsite == nil {
		return []string{"PGBACKREST_EXPIRE_AUTO=n"}, nil
	}
	return e.repo2Env(e.serviceKeysDir())
}

// repo2Env is the repo2 environment with the S3 keys of keysDir.
func (e *Executor) repo2Env(keysDir string) ([]string, error) {
	t := e.cfg.Offsite
	if t == nil {
		return nil, execError(codeOffsiteNotConfigured)
	}
	keys, err := readOffsiteKeys(keysDir)
	if err != nil {
		return nil, err
	}
	cipher, err := readSecret(e.cfg.SecretsDir, fileCipherPass)
	if err != nil {
		return nil, err
	}
	style := "host"
	if e.cfg.OffsitePathStyle {
		style = "path"
	}
	env := []string{
		"PGBACKREST_REPO2_TYPE=s3",
		"PGBACKREST_REPO2_PATH=/" + t.Key("pgbackrest"),
		"PGBACKREST_REPO2_S3_BUCKET=" + t.Bucket,
		"PGBACKREST_REPO2_S3_ENDPOINT=" + t.Host,
		"PGBACKREST_REPO2_STORAGE_PORT=" + strconv.Itoa(t.Port),
		"PGBACKREST_REPO2_S3_REGION=" + e.cfg.OffsiteRegion,
		"PGBACKREST_REPO2_S3_URI_STYLE=" + style,
		"PGBACKREST_REPO2_S3_KEY=" + keys.access,
		"PGBACKREST_REPO2_S3_KEY_SECRET=" + keys.secret,
		"PGBACKREST_REPO2_CIPHER_TYPE=aes-256-cbc",
		"PGBACKREST_REPO2_CIPHER_PASS=" + cipher,
		"PGBACKREST_EXPIRE_AUTO=n",
	}
	if keys.ca != "" {
		env = append(env, "PGBACKREST_REPO2_STORAGE_CA_FILE="+keys.ca)
	}
	return env, nil
}

// pgbackrest runs pgbackrest with --stanza first and env; stdout is returned for info.
func (e *Executor) pgbackrest(ctx context.Context, env []string, args ...string) ([]byte, error) {
	return e.run(ctx, toolPgBackRest, e.cfg.PgBackRest, append([]string{"--stanza=" + e.cfg.Stanza}, args...), env, nil)
}

// PgBackRestBackup implements app.Executor: "pgbackrest backup" of req.Type into req.Repo, then
// "pgbackrest info" of that repository for the label and the repository delta of the newest
// backup.
func (e *Executor) PgBackRestBackup(ctx context.Context, req app.PgBackRestBackup) (app.PgBackRestResult, error) {
	if (req.Type != "full" && req.Type != "diff") || (req.Repo != 1 && req.Repo != 2) {
		return app.PgBackRestResult{}, execError(codePgBackRestRequestInvalid)
	}
	if req.Repo == 2 && e.cfg.Offsite == nil {
		return app.PgBackRestResult{}, execError(codeOffsiteNotConfigured)
	}
	env, err := e.PgBackRestEnv(ctx)
	if err != nil {
		return app.PgBackRestResult{}, err
	}
	repo := "--repo=" + strconv.Itoa(req.Repo)
	if _, err := e.pgbackrest(ctx, env, repo, "--type="+req.Type, "backup"); err != nil {
		return app.PgBackRestResult{}, err
	}
	out, err := e.pgbackrest(ctx, env, repo, "--output=json", "info")
	if err != nil {
		return app.PgBackRestResult{}, err
	}
	return e.newestBackup(out)
}

// infoStanza is the part of one stanza of "pgbackrest info --output=json" the executor reads.
type infoStanza struct {
	Name   string `json:"name"`
	Backup []struct {
		Label     string `json:"label"`
		Timestamp struct {
			Stop int64 `json:"stop"`
		} `json:"timestamp"`
		Info struct {
			Repository struct {
				Delta *int64 `json:"delta"`
			} `json:"repository"`
		} `json:"info"`
	} `json:"backup"`
}

// newestBackup returns the backup of the stanza with the latest stop time (the last one on a
// tie, as info lists them oldest first). The error is pgbackrest_output_invalid.
func (e *Executor) newestBackup(out []byte) (app.PgBackRestResult, error) {
	invalid := execError(toolPgBackRest + "_output_invalid")
	var stanzas []infoStanza
	if err := json.Unmarshal(out, &stanzas); err != nil {
		return app.PgBackRestResult{}, invalid
	}
	for _, s := range stanzas {
		if s.Name != e.cfg.Stanza || len(s.Backup) == 0 {
			continue
		}
		best := 0
		for i, b := range s.Backup {
			if b.Timestamp.Stop >= s.Backup[best].Timestamp.Stop {
				best = i
			}
		}
		b := s.Backup[best]
		if !labelPattern.MatchString(b.Label) || b.Info.Repository.Delta == nil || *b.Info.Repository.Delta < 0 {
			return app.PgBackRestResult{}, invalid
		}
		return app.PgBackRestResult{Label: b.Label, SizeBytes: *b.Info.Repository.Delta}, nil
	}
	return app.PgBackRestResult{}, invalid
}

// PgBackRestExpireLocal implements app.Executor: "pgbackrest expire" on repo1 only.
func (e *Executor) PgBackRestExpireLocal(ctx context.Context, keepFull int) error {
	if keepFull < 1 {
		return execError(codePgBackRestRequestInvalid)
	}
	env, err := e.PgBackRestEnv(ctx)
	if err != nil {
		return err
	}
	_, err = e.pgbackrest(ctx, env, "--repo=1", "--repo1-retention-full="+strconv.Itoa(keepFull), "expire")
	return err
}

// PgBackRestExpireOffsite implements app.Executor: "pgbackrest expire" on repo2 with the keys
// of the owner's credentials directory adminDir instead of offsite/ (R-OP-09).
func (e *Executor) PgBackRestExpireOffsite(ctx context.Context, adminDir string, keepFull int) error {
	if keepFull < 1 || adminDir == "" {
		return execError(codePgBackRestRequestInvalid)
	}
	env, err := e.repo2Env(adminDir)
	if err != nil {
		return err
	}
	_, err = e.pgbackrest(ctx, env, "--repo=2", "--repo2-retention-full="+strconv.Itoa(keepFull), "expire")
	return err
}
