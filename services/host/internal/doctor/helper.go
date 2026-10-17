// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// Modes of a run: Full at the start of the service and on command (with the entry scan of HD11),
// Light before every PTY and every host_service.credential_check_interval (M2-45).
const (
	ModeFull  = "full"
	ModeLight = "light"
)

// ChainFiles are the entries the coding agent or git load from folders above a working directory
// (24 2.2 point 6, checked against the Claude Code pages memory, settings and mcp and the git page
// on 2026-10-15, docs/DECISIONS.md D-M2-42-10).
var ChainFiles = []string{"CLAUDE.md", "CLAUDE.local.md", "AGENTS.md", ".mcp.json", ".claude", ".git"}

// OrgConfigDir is the CLAUDE_CONFIG_DIR of the profile claude at organization, relative to the
// home of the session user (24 7.4).
var OrgConfigDir = filepath.Join(".config", "bothy-host", "claude")

// NamedHome is the home of an account HD9 lists.
type NamedHome struct {
	Name string `json:"name"`
	Home string `json:"home"`
}

// Request is the op check.run of the local channel: what a helper checks for its session user.
// It carries paths of the host only over the local channel, never to the server.
type Request struct {
	Mode        string `json:"mode"`
	SessionUser string `json:"session_user"`
	// Others are the other session users; ServiceUser is the service user.
	Others      []string `json:"others"`
	ServiceUser string   `json:"service_user"`
	// ClaudeBin is the resolved host_service.claude_bin; empty when it is empty or counts as empty.
	ClaudeBin  string   `json:"claude_bin"`
	SearchList []string `json:"search_list"`
	// Declaration is the declaration in use (none, personal, organization or unknown).
	Declaration string `json:"declaration"`
	// Workdirs are the configured working directories of the session user, in order.
	Workdirs     []string    `json:"workdirs"`
	ForeignHomes []NamedHome `json:"foreign_homes"`
	StateDir     string      `json:"state_dir"`
	KeyFile      string      `json:"key_file"`
	// AuthStatusTimeout and EntryScanTimeout are host_service.auth_status_timeout and
	// host_service.entry_scan_timeout.
	AuthStatusTimeout time.Duration `json:"auth_status_timeout"`
	EntryScanTimeout  time.Duration `json:"entry_scan_timeout"`
}

// LLMFacts are the facts of HD6 in one session user; the service evaluates them with the
// declaration, so a new declaration needs no new run.
type LLMFacts struct {
	// Finding: authMethod claude.ai or oauth_token, a login file, or CLAUDE_CODE_OAUTH_TOKEN set.
	Finding bool `json:"finding"`
	// Unknown: an unreadable output, a timeout or an exit code other than 0 and 1 (fail closed).
	Unknown bool `json:"unknown"`
	// Unchecked: host_service.claude_bin is empty or counts as empty; only the locations were asked.
	Unchecked bool `json:"unchecked"`
}

// HelperResult is the op check.result: codes, names and identities only.
type HelperResult struct {
	Sudo       bool     `json:"sudo"`
	LLM        LLMFacts `json:"llm"`
	ReadsState bool     `json:"reads_state"`
	// Listable are the names of the accounts of ForeignHomes whose home the helper could list.
	Listable []string `json:"listable"`
	// PlaceCode is the HD11 code of the places of the session user, empty when they pass.
	PlaceCode string `json:"place_code"`
	// WorkdirIDs holds the identity of each working directory of the request, empty when the
	// helper cannot resolve it.
	WorkdirIDs []string `json:"workdir_ids"`
}

// HelperEnv is the operating system as the helper of one session user sees it.
type HelperEnv struct {
	GOOS     string
	Self     hostos.Account
	Users    hostos.Users
	Security hostos.Security
	Resolver hostos.Resolver
	Files    hostos.Lister
	Probe    hostos.Probe
	Runner   hostos.Runner
	Clock    clock.Clock
	// LoginEnv reports whether the login shell of the session user (Windows: its environment in the
	// registry hive) sets CLAUDE_CODE_OAUTH_TOKEN, never its value, and the value of
	// CLAUDE_CONFIG_DIR it exports ("" when unset); the folder stays in the helper.
	LoginEnv func(ctx context.Context, timeout time.Duration) (tokenSet bool, configDir string, err error)
	// MkdirPrivate creates a folder only the session user can enter (mode 0700, on Windows an ACL only
	// for it) if it is missing.
	MkdirPrivate func(path string) error
}

// RunHelper runs the helper-side checks of req as the session user: HD3, HD6, the helper part of
// HD8, HD9, the places of HD11 and the identities of the working directories for HD7.
func RunHelper(ctx context.Context, env HelperEnv, req Request) HelperResult {
	var res HelperResult
	if env.GOOS != hostos.Windows {
		ok, err := env.Probe.SudoWithoutPassword(ctx)
		res.Sudo = ok || err != nil
	}
	res.LLM = checkLLM(ctx, env, req)
	res.ReadsState = readsState(env, req)
	for _, h := range req.ForeignHomes {
		if _, err := env.Files.ReadDir(h.Home); err == nil {
			res.Listable = append(res.Listable, h.Name)
		}
	}
	others := subjects(env.Users, req.Others)
	res.PlaceCode = checkPlaces(env, req, others)
	for _, w := range req.Workdirs {
		id, err := env.Resolver.Identity(w)
		if err != nil {
			id = ""
		}
		res.WorkdirIDs = append(res.WorkdirIDs, id)
	}
	return res
}

func readsState(env HelperEnv, req Request) bool {
	if req.StateDir != "" {
		if _, err := env.Files.ReadDir(req.StateDir); err == nil {
			return true
		}
	}
	if req.KeyFile != "" {
		if _, err := env.Files.Head(req.KeyFile, 1); err == nil {
			return true
		}
	}
	return false
}

// subjects returns the evaluator view of each named account that exists.
func subjects(users hostos.Users, names []string) []hostos.Subject {
	var out []hostos.Subject
	for _, n := range names {
		a, err := users.Lookup(n)
		if err != nil {
			continue
		}
		s := hostos.Subject{Name: n, ID: a.ID}
		if gs, err := users.Groups(n); err == nil {
			for _, g := range gs {
				s.GroupIDs = append(s.GroupIDs, g.ID)
			}
		}
		out = append(out, s)
	}
	return out
}

func (env HelperEnv) orgConfigDir() string { return filepath.Join(env.Self.Home, OrgConfigDir) }

// authMethods of claude auth status (24 7.4).
var authMethods = []string{"none", "claude.ai", "oauth_token", "api_key", "api_key_helper", "third_party"}

// checkLLM gathers the facts of HD6 (24 7.4, D-M2-42-9).
func checkLLM(ctx context.Context, env HelperEnv, req Request) LLMFacts {
	var f LLMFacts
	org := env.orgConfigDir()
	if env.MkdirPrivate != nil {
		_ = env.MkdirPrivate(org) // a failure shows as a missing folder in the checks below
	}
	dirs := []string{"", org}
	if exported := loginShell(ctx, env, req, &f); exported != "" && !slices.Contains(dirs, exported) {
		dirs = append(dirs, exported)
	}
	if req.ClaudeBin == "" {
		f.Unchecked = true
	} else {
		for _, cfg := range dirs {
			finding, unknown := authStatus(ctx, env, req, cfg)
			f.Finding = f.Finding || finding
			f.Unknown = f.Unknown || unknown
		}
	}
	files := []string{filepath.Join(env.Self.Home, ".claude", ".credentials.json")}
	for _, d := range dirs[1:] {
		files = append(files, filepath.Join(d, ".credentials.json"))
	}
	for _, file := range files {
		if _, err := env.Security.Lstat(file); err == nil {
			f.Finding = true
		}
	}
	return f
}

// loginShell runs the login shell probe and returns the CLAUDE_CONFIG_DIR it exports.
func loginShell(ctx context.Context, env HelperEnv, req Request, f *LLMFacts) string {
	if env.LoginEnv == nil {
		return ""
	}
	set, exported, err := env.LoginEnv(ctx, req.AuthStatusTimeout)
	f.Finding = f.Finding || set
	f.Unknown = f.Unknown || err != nil
	return exported
}

// authStatus runs "claude auth status" once with the environment of 7.4 and reads only authMethod.
func authStatus(ctx context.Context, env HelperEnv, req Request, configDir string) (finding, unknown bool) {
	homeVar := "HOME"
	sep := ":"
	if env.GOOS == hostos.Windows {
		homeVar, sep = "USERPROFILE", ";"
	}
	vars := []string{homeVar + "=" + env.Self.Home, "PATH=" + strings.Join(req.SearchList, sep)}
	if configDir != "" {
		vars = append(vars, "CLAUDE_CONFIG_DIR="+configDir)
	}
	out, err := env.Runner.Run(ctx, hostos.Cmd{Path: req.ClaudeBin, Args: []string{"auth", "status"}, Env: vars, Dir: env.Self.Home, Timeout: req.AuthStatusTimeout})
	if err != nil || out.TimedOut || (out.ExitCode != 0 && out.ExitCode != 1) {
		return false, true
	}
	var v struct {
		AuthMethod *string `json:"authMethod"`
	}
	if json.Unmarshal(out.Stdout, &v) != nil || v.AuthMethod == nil || !slices.Contains(authMethods, *v.AuthMethod) {
		return false, true
	}
	return *v.AuthMethod == "claude.ai" || *v.AuthMethod == "oauth_token", false
}

// checkPlaces checks the places of the session user (HD11): home, the CLAUDE_CONFIG_DIR of
// organization and the working directories.
func checkPlaces(env HelperEnv, req Request, others []hostos.Subject) string {
	if _, err := env.Resolver.Resolve(env.Self.Home); err != nil {
		return CodeHomeUnresolvable
	}
	places := append([]string{env.Self.Home, env.orgConfigDir()}, req.Workdirs...)
	for _, p := range places {
		r, err := env.Resolver.Resolve(p)
		if err != nil {
			continue // a missing folder is not offered (HD7); the home was checked above
		}
		if !placeOfSessionUserOK(env, r, others) {
			return CodeSharedLocation
		}
	}
	return ""
}

// placeOfSessionUserOK applies the rule of a place of a session user (24 2.2 point 6): the
// place, every folder on the way, every configured component, every link with its folder and the
// chain files in those folders belong to no other session user and are writable by none; the
// place itself is not readable by another.
func placeOfSessionUserOK(env HelperEnv, r hostos.Resolved, others []hostos.Subject) bool {
	objects := slices.Concat(r.Chain, r.Components, []string{r.Final})
	folders := slices.Concat(r.Chain, []string{r.Final})
	for _, l := range r.Links {
		objects = append(objects, l.Path, l.Dir)
		folders = append(folders, l.Dir)
	}
	for _, o := range objects {
		sec, err := env.Security.Lstat(o)
		if err != nil || !notOthers(sec, others) {
			return false
		}
	}
	for _, d := range folders {
		for _, name := range ChainFiles {
			if !chainEntryOK(env, filepath.Join(d, name), others, 0) {
				return false
			}
		}
	}
	final, err := env.Security.Lstat(r.Final)
	if err != nil {
		return false
	}
	for _, s := range others {
		if hostos.Readable(final, s) {
			return false
		}
	}
	return true
}

func notOthers(sec hostos.FileSec, others []hostos.Subject) bool {
	for _, s := range others {
		if sec.OwnerID == s.ID || hostos.Writable(sec, s) {
			return false
		}
	}
	return true
}

// maxChainDepth bounds the walk into .claude and .git; deeper entries count as a finding (fail
// closed). It is a protocol constant of the walk, not a product window.
const maxChainDepth = 64

// chainEntryOK checks one chain file and, for .claude and .git, everything in it.
func chainEntryOK(env HelperEnv, p string, others []hostos.Subject, depth int) bool {
	sec, err := env.Security.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil || depth > maxChainDepth || !notOthers(sec, others) {
		return false
	}
	if sec.IsLink {
		// A link counts as its target too (24 2.2 point 6): the target, its folders and links
		// belong to no other session user; a dangling link fails, because another could create
		// its target later.
		return linkTargetOK(env, p, others, depth)
	}
	if !sec.IsDir {
		return true
	}
	names, err := env.Files.ReadDir(p)
	if err != nil {
		return false
	}
	for _, n := range names {
		if !chainEntryOK(env, filepath.Join(p, n), others, depth+1) {
			return false
		}
	}
	return true
}

// linkTargetOK checks the target of the chain file link p like a chain file of its own.
func linkTargetOK(env HelperEnv, p string, others []hostos.Subject, depth int) bool {
	r, err := env.Resolver.Resolve(p)
	if err != nil {
		return false
	}
	objects := slices.Concat(r.Chain, []string{r.Final})
	for _, l := range r.Links {
		objects = append(objects, l.Path, l.Dir)
	}
	for _, o := range objects {
		sec, err := env.Security.Lstat(o)
		if err != nil || !notOthers(sec, others) {
			return false
		}
	}
	return chainEntryOK(env, r.Final, others, depth+1)
}

// ScanEntries is the entry scan of HD11 in the helper (Full only): no entry in a place of the
// session user belongs to another session user. It returns "", foreign_entry or
// entry_scan_timeout when req.EntryScanTimeout passes first (24 2.2 "Check of the entries").
func ScanEntries(ctx context.Context, env HelperEnv, req Request) string {
	others := subjects(env.Users, req.Others)
	ids := map[string]bool{}
	for _, s := range others {
		ids[s.ID] = true
	}
	timer := env.Clock.NewTimer(req.EntryScanTimeout)
	defer timer.Stop()
	var queue []string
	for _, p := range append([]string{env.Self.Home, env.orgConfigDir()}, req.Workdirs...) {
		if r, err := env.Resolver.Resolve(p); err == nil {
			queue = append(queue, r.Final)
		}
	}
	seen := map[string]bool{}
	for len(queue) > 0 {
		select {
		case <-timer.C():
			return CodeEntryScanTimeout
		case <-ctx.Done():
			return CodeEntryScanTimeout
		default:
		}
		dir := queue[0]
		queue = queue[1:]
		if seen[dir] {
			continue
		}
		seen[dir] = true
		sub, foreign := scanFolder(env, dir, ids)
		if foreign {
			return CodeForeignEntry
		}
		queue = append(queue, sub...)
	}
	return ""
}

// scanFolder lists dir and returns its subfolders, or foreign when an entry belongs to one of the
// accounts in ids. Links are entries, never followed.
func scanFolder(env HelperEnv, dir string, ids map[string]bool) (sub []string, foreign bool) {
	names, err := env.Files.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	for _, n := range names {
		p := filepath.Join(dir, n)
		sec, err := env.Security.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil, ids[sec.OwnerID]:
			// An entry whose owner cannot be read counts as foreign (fail closed).
			return nil, true
		case sec.IsDir && !sec.IsLink:
			sub = append(sub, p)
		}
	}
	return sub, false
}
