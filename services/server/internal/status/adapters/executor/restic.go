// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// codeResticRequestInvalid reports a request the executor refuses before running restic.
const codeResticRequestInvalid = "restic_request_invalid"

// resticNoRepository is the exit code of restic 0.19 when the repository does not exist.
const resticNoRepository = 10

// snapshotPattern is a full restic snapshot id.
var snapshotPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// resticRepo returns the repository flags and the environment of the restic repository of the
// instance: off-site with the S3 keys of keysDir when an off-site target is set, else the local
// repository. The password file is always <secrets.dir>/restic_password.
func (e *Executor) resticRepo(keysDir string) ([]string, []string, error) {
	env := []string{"RESTIC_PASSWORD_FILE=" + filepath.Join(e.cfg.SecretsDir, fileResticPassword)}
	t := e.cfg.Offsite
	if t == nil {
		return []string{"-r", e.cfg.LocalResticRepo}, env, nil
	}
	keys, err := readOffsiteKeys(keysDir)
	if err != nil {
		return nil, nil, err
	}
	lookup := "dns"
	if e.cfg.OffsitePathStyle {
		lookup = "path"
	}
	args := []string{"-r", "s3:" + t.Endpoint() + "/" + t.Bucket + "/" + t.Key("restic"),
		"-o", "s3.bucket-lookup=" + lookup, "-o", "s3.region=" + e.cfg.OffsiteRegion}
	if keys.ca != "" {
		args = append(args, "--cacert", keys.ca)
	}
	env = append(env, "AWS_ACCESS_KEY_ID="+keys.access, "AWS_SECRET_ACCESS_KEY="+keys.secret)
	return args, env, nil
}

// restic runs restic with the repository flags first.
func (e *Executor) restic(ctx context.Context, repo, env []string, stdin []byte, args ...string) ([]byte, error) {
	var in *bytes.Reader
	if stdin != nil {
		in = bytes.NewReader(stdin)
	}
	all := append(append(make([]string, 0, len(repo)+len(args)), repo...), args...)
	if in == nil {
		return e.run(ctx, toolRestic, e.cfg.Restic, all, env, nil)
	}
	return e.run(ctx, toolRestic, e.cfg.Restic, all, env, in)
}

// ensureRepo initialises the repository when "restic cat config" exits 10 (no repository).
func (e *Executor) ensureRepo(ctx context.Context, repo, env []string) error {
	_, err := e.restic(ctx, repo, env, nil, "cat", "config")
	var ee *app.ExecError
	if errors.As(err, &ee) && ee.Code == toolRestic+"_exit_"+strconv.Itoa(resticNoRepository) {
		_, err = e.restic(ctx, repo, env, nil, "init")
	}
	return err
}

// ResticBackup implements app.Executor: the repository is initialised on first use, then
// "restic backup --json" of req.Paths or of req.Stdin; the summary line gives the snapshot id and
// data_added_packed.
func (e *Executor) ResticBackup(ctx context.Context, req app.ResticBackup) (app.ResticResult, error) {
	if (req.Stdin == nil && len(req.Paths) == 0) || (req.Stdin != nil && req.StdinName == "") {
		return app.ResticResult{}, execError(codeResticRequestInvalid)
	}
	repo, env, err := e.resticRepo(e.serviceKeysDir())
	if err != nil {
		return app.ResticResult{}, err
	}
	if err := e.ensureRepo(ctx, repo, env); err != nil {
		return app.ResticResult{}, err
	}
	// --quiet leaves only the summary on stdout.
	args := []string{"backup", "--json", "--quiet"}
	for _, tag := range req.Tags {
		args = append(args, "--tag", tag)
	}
	if req.Stdin != nil {
		args = append(args, "--stdin", "--stdin-filename", req.StdinName)
	} else {
		// "--" keeps a path that starts with "-" from being read as a flag.
		args = append(append(args, "--"), req.Paths...)
	}
	out, err := e.restic(ctx, repo, env, req.Stdin, args...)
	if err != nil {
		return app.ResticResult{}, err
	}
	return parseSummary(out)
}

// parseSummary reads the summary line of restic backup --json. The error is
// restic_output_invalid.
func parseSummary(out []byte) (app.ResticResult, error) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var line struct {
			MessageType     string `json:"message_type"`
			SnapshotID      string `json:"snapshot_id"`
			DataAddedPacked *int64 `json:"data_added_packed"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil || line.MessageType != "summary" {
			continue
		}
		if !snapshotPattern.MatchString(line.SnapshotID) || line.DataAddedPacked == nil || *line.DataAddedPacked < 0 {
			break
		}
		return app.ResticResult{SnapshotID: line.SnapshotID, SizeBytes: *line.DataAddedPacked}, nil
	}
	return app.ResticResult{}, execError(toolRestic + "_output_invalid")
}

// ResticForget implements app.Executor: "restic forget --tag <tag> --keep-within <N>h --prune"
// with KeepWithin rounded up to whole hours. All snapshots of the tag form one group
// (--group-by=), because keep-within counts from the newest snapshot of a group and every
// manifest snapshot has its own file name. With req.AdminDir the off-site repository with the
// owner's keys; without, only the local repository: the service never deletes off-site, so an
// off-site target without AdminDir is restic_offsite_forget_refused.
func (e *Executor) ResticForget(ctx context.Context, req app.ResticForget) error {
	hours := int64((req.KeepWithin + time.Hour - 1) / time.Hour)
	if req.Tag == "" || hours < 1 {
		return execError(codeResticRequestInvalid)
	}
	keysDir := req.AdminDir
	switch {
	case req.AdminDir != "" && e.cfg.Offsite == nil:
		return execError(codeOffsiteNotConfigured)
	case req.AdminDir == "" && e.cfg.Offsite != nil:
		return execError(codeOffsiteForgetRefused)
	}
	repo, env, err := e.resticRepo(keysDir)
	if err != nil {
		return err
	}
	_, err = e.restic(ctx, repo, env, nil, "forget", "--tag", req.Tag, "--group-by=",
		"--keep-within", strconv.FormatInt(hours, 10)+"h", "--prune")
	return err
}
