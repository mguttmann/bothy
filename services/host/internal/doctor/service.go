// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Helpers reaches the PTY helper of each session user over its local channel.
type Helpers interface {
	// Check runs check.run in the helper of sessionUser. An error wrapping ErrPeerMismatch means
	// the peer check of the channel failed and no message was sent; any other error means the
	// helper could not be reached.
	Check(ctx context.Context, sessionUser string, req Request) (HelperResult, error)
	// Scan runs the entry scan in the helper of sessionUser and returns its code.
	Scan(ctx context.Context, sessionUser string, req Request) (string, error)
}

// ErrPeerMismatch is the failure of the peer check of a local channel (24 2.2 PTY helper point 3).
var ErrPeerMismatch = errors.New("doctor: peer of the local channel is not the helper")

// CodesignInfo is what codesign reports about a macOS binary.
type CodesignInfo struct {
	// Runtime tells that the signature carries the hardened runtime flag.
	Runtime bool
	// GetTaskAllow tells that the entitlements contain com.apple.security.get-task-allow.
	GetTaskAllow bool
}

// Env is everything the service-side checks read.
type Env struct {
	GOOS        string
	Config      *config.Config
	ServiceUser string
	// StateDir is host_service.state_dir; KeyFile the key file in it.
	StateDir, KeyFile string
	// Declaration is the stored declaration (state.DeclUnknown when never received).
	Declaration string
	// Bindings are the bindings of session users to persons from the state directory.
	Bindings map[string]string
	// ServerURL is the server of the pairing record, empty when not paired.
	ServerURL string

	Users    hostos.Users
	Security hostos.Security
	Resolver hostos.Resolver
	Files    hostos.Lister
	Probe    hostos.Probe
	Helpers  Helpers

	// HelperBinary is the installed program; ServiceDefinitions the installed unit files, plist
	// files (Windows: none, the service control manager holds them); EndpointDirs the folders of
	// the channel endpoints.
	HelperBinary       string
	ServiceDefinitions []string
	EndpointDirs       []string
	// SystemSearchDirs is the fixed tail of the search list of HD6: /usr/bin and /bin, on Windows
	// %SystemRoot%\System32.
	SystemSearchDirs []string
	// SystemDrive is the system drive on Windows ("C:").
	SystemDrive string
	// DockerSocket is the socket of the Docker daemon checked on Linux.
	DockerSocket string
	// PeerCheckProven tells that the platform run of TM-T26 proved the peer check here.
	PeerCheckProven bool
	// Codesign reads the signature flags of a macOS binary.
	Codesign func(ctx context.Context, binary string) (CodesignInfo, error)
	// HelperGrace is added to the bound of every helper call (zero means helperGrace): a helper
	// that does not answer in time counts as unreachable, so no session user can hold the run.
	HelperGrace time.Duration
	// Mode is the mode of the check.run requests (ModeFull when empty); Runner sets ModeLight for
	// RunLight and RunLightUser.
	Mode string
	// InterfaceAddrs and LookupIP serve HD5.
	InterfaceAddrs func() ([]net.IP, error)
	LookupIP       func(ctx context.Context, host string) ([]net.IP, error)
}

// helperGrace is the default HelperGrace, a protocol constant of the channel, not a product
// window; it also covers the sudo probe of the helper.
const helperGrace = 30 * time.Second

func (e Env) grace() time.Duration {
	if e.HelperGrace > 0 {
		return e.HelperGrace
	}
	return helperGrace
}

// checkBound bounds one check.run: up to three calls of claude auth status and the login shell
// probe, each bounded by host_service.auth_status_timeout, plus the grace.
func (e Env) checkBound() time.Duration { return 4*e.Config.AuthStatusTimeout + e.grace() }

// scanBound bounds one scan.run: host_service.entry_scan_timeout plus the grace.
func (e Env) scanBound() time.Duration { return e.Config.EntryScanTimeout + e.grace() }

// ForbiddenGroups is the fixed list of HD2; host_service.extra_forbidden_groups extends it and
// cannot shorten it.
var ForbiddenGroups = []string{"admin", "sudo", "wheel", "Administrators", "docker", "lxd", "libvirt", "disk"}

// privilegedNames are the names HD1 refuses for every user of the host service.
var privilegedNames = []string{"root", "Administrator", "SYSTEM"}

// osUserPattern is the pattern of HostReportedSessionUser.os_user.
var osUserPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// facts are the inputs that a new declaration re-evaluates without a new run.
type facts struct {
	llm            map[string]LLMFacts
	claudeBinEmpty bool
	profiles       []config.Profile
	droppedProfile []string
	sessionUsers   []string
}

// run is one service-side run in progress.
type run struct {
	env       Env
	res       *Result
	facts     facts
	users     []string // configured session users that passed HD1's per-name rules and exist
	// workdirCodes are the HD11 codes of the working directories per session user, by path.
	workdirCodes map[string]map[string]string
	subjects  []hostos.Subject
	claudeBin string
	search    []string
	former    []string
}

// RunService runs HD1 to HD11 as the service user in mode (ModeFull or ModeLight), asking each
// helper over its channel. The entry scan is not part of it; Runner adds it.
func RunService(ctx context.Context, env Env) *Result {
	r := &run{env: env, res: &Result{Declaration: env.Declaration, Workdirs: map[string][]Workdir{}}}
	for _, name := range Checks {
		r.res.Checks = append(r.res.Checks, CheckResult{Check: name})
	}
	r.hd1()
	r.subjects = subjects(env.Users, r.users)
	r.hd2()
	r.hd4()
	r.hd5(ctx)
	r.hd8Service()
	r.hd10(ctx)
	r.hd7Profiles()
	r.placesForAll()
	results := r.askHelpers(ctx)
	r.aggregate(results)
	r.res.facts = r.facts
	r.res.ClaudeBin, r.res.SearchList = r.claudeBin, slices.Clone(r.search)
	r.res.evaluate(env.Declaration)
	return r.res
}

// add appends e to check; an entry with the same status, code and session user merges its local
// names into the existing one.
func (r *run) add(check string, e Entry) {
	i := slices.IndexFunc(r.res.Checks, func(c CheckResult) bool { return c.Check == check })
	if i < 0 {
		return
	}
	c := &r.res.Checks[i]
	j := slices.IndexFunc(c.Entries, func(o Entry) bool { return o.Status == e.Status && o.Code == e.Code && o.SessionUser == e.SessionUser })
	if j < 0 {
		c.Entries = append(c.Entries, e)
		return
	}
	o := &c.Entries[j]
	for _, l := range e.Local {
		if !slices.Contains(o.Local, l) {
			o.Local = append(o.Local, l)
		}
	}
}

func (r *run) fail(check, code string, local ...string) {
	r.add(check, Entry{Status: Fail, Code: code, Local: local})
}

func foldEq(a, b string) bool { return strings.EqualFold(a, b) }

func (r *run) hd1() {
	c := r.env.Config
	if len(c.SessionUsers) == 0 {
		r.fail(HD1, CodeSessionUsersEmpty)
	}
	for _, n := range privilegedNames {
		if foldEq(r.env.ServiceUser, n) {
			r.fail(HD1, CodePrivilegedUser)
		}
	}
	if a, err := r.env.Users.Lookup(r.env.ServiceUser); err != nil {
		r.fail(HD1, CodeUserMissing)
	} else if a.ID == "0" {
		r.fail(HD1, CodePrivilegedUser)
	}
	for i, u := range c.SessionUsers {
		codes := r.sessionUserCodes(u, c.SessionUsers[:i])
		for _, code := range codes {
			r.fail(HD1, code)
		}
		if len(codes) == 0 {
			r.users = append(r.users, u)
		}
	}
}

// sessionUserCodes returns the HD1 codes of the session user u; earlier are the names before it.
func (r *run) sessionUserCodes(u string, earlier []string) []string {
	var codes []string
	if slices.ContainsFunc(earlier, func(o string) bool { return foldEq(u, o) }) {
		codes = append(codes, CodeSessionUserDuplicate)
	}
	if foldEq(u, r.env.ServiceUser) {
		codes = append(codes, CodeSessionUserIsService)
	}
	if slices.ContainsFunc(privilegedNames, func(n string) bool { return foldEq(n, u) }) {
		codes = append(codes, CodePrivilegedUser)
	}
	if !osUserPattern.MatchString(u) {
		codes = append(codes, CodeSessionUserNameInvalid)
	}
	a, err := r.env.Users.Lookup(u)
	switch {
	case err != nil:
		codes = append(codes, CodeUserMissing)
	case a.ID == "0":
		codes = append(codes, CodePrivilegedUser)
	}
	return codes
}

func (r *run) hd2() {
	forbidden := slices.Concat(ForbiddenGroups, r.env.Config.ExtraForbiddenGroups)
	match := func(a, b string) bool {
		if r.env.GOOS == hostos.Windows {
			return foldEq(a, b)
		}
		return a == b
	}
	all := append([]string{r.env.ServiceUser}, r.users...)
	for _, u := range all {
		groups, err := r.env.Users.Groups(u)
		// Groups that cannot be read count as forbidden (fail closed).
		// Windows names groups in the language of the system; Administrators is also matched by its SID.
		if err != nil || slices.ContainsFunc(groups, func(g hostos.Group) bool {
			return (r.env.GOOS == hostos.Windows && g.ID == hostos.SIDAdministrators) ||
				slices.ContainsFunc(forbidden, func(f string) bool { return match(f, g.Name) })
		}) {
			r.fail(HD2, CodeForbiddenGroup)
		}
	}
	if r.env.GOOS == hostos.Linux && r.env.DockerSocket != "" {
		r.dockerSocket()
	}
}

// dockerSocket checks that the socket of the Docker daemon is writable by no user of the host
// service (Linux).
func (r *run) dockerSocket() {
	res, err := r.env.Resolver.Resolve(r.env.DockerSocket)
	if err != nil {
		return // no Docker daemon socket
	}
	sec, err := r.env.Security.Lstat(res.Final)
	if err != nil {
		return
	}
	for _, s := range append(subjects(r.env.Users, []string{r.env.ServiceUser}), r.subjects...) {
		if hostos.Writable(sec, s) {
			r.fail(HD2, CodeDockerSocketWritable)
		}
	}
}

// dpapiHeader is the start of every DPAPI blob: version 1 and the provider GUID
// df9d8cd0-1501-11d1-8c7a-00c04fc297eb.
var dpapiHeader = []byte{1, 0, 0, 0, 0xd0, 0x8c, 0x9d, 0xdf, 0x01, 0x15, 0xd1, 0x11, 0x8c, 0x7a, 0x00, 0xc0, 0x4f, 0xc2, 0x97, 0xeb}

func (r *run) hd4() {
	svc, err := r.env.Users.Lookup(r.env.ServiceUser)
	sec, serr := r.env.Security.Lstat(r.env.StateDir)
	// On Windows install cannot hand the folder to the virtual account without a restore
	// privilege; SYSTEM or Administrators as owner with the protected DACL are equal in trust.
	ownerOK := sec.OwnerID == svc.ID || (r.env.GOOS == hostos.Windows && hostos.TrustedOwner(hostos.Windows, sec) && sec.OwnerID != hostos.SIDTrustedInstall)
	if err != nil || serr != nil || !ownerOK {
		r.fail(HD4, CodeStateDirOwner)
		return
	}
	if r.env.GOOS == hostos.Windows && !onlyPrincipals(sec, svc.ID) || r.env.GOOS != hostos.Windows && (sec.Mode.Perm() != 0o700 || len(sec.ACL) > 0) {
		r.fail(HD4, CodeStateDirMode)
	}
	if !r.keyFileProtected(svc.ID) {
		r.fail(HD4, CodeKeyFileUnprotected)
	}
}

// keyFileProtected checks the key file: mode 0600 of the service user, on Windows a DPAPI blob with
// the three principals; a missing file (not paired yet) passes.
func (r *run) keyFileProtected(service string) bool {
	key, err := r.env.Security.Lstat(r.env.KeyFile)
	if err != nil {
		return true
	}
	if r.env.GOOS == hostos.Windows {
		head, herr := r.env.Files.Head(r.env.KeyFile, len(dpapiHeader))
		return herr == nil && bytes.Equal(head, dpapiHeader) && onlyPrincipals(key, service)
	}
	return key.Mode.Perm() == 0o600 && key.OwnerID == service && len(key.ACL) == 0
}

// onlyPrincipals reports whether every allow entry of f names the service account, SYSTEM or
// Administrators (the protected DACL of 4.2).
func onlyPrincipals(f hostos.FileSec, service string) bool {
	for _, e := range f.ACL {
		if e.Unmappable {
			return false
		}
		if e.Allow && e.ID != service && e.ID != hostos.SIDSystem && e.ID != hostos.SIDAdministrators {
			return false
		}
	}
	return true
}

func (r *run) hd5(ctx context.Context) {
	if r.env.ServerURL == "" {
		return
	}
	u, err := url.Parse(r.env.ServerURL)
	if err != nil || u.Hostname() == "" {
		return
	}
	var ips []net.IP
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		ips = []net.IP{ip}
	} else if r.env.LookupIP != nil {
		ips, _ = r.env.LookupIP(ctx, u.Hostname())
	}
	var local []net.IP
	if r.env.InterfaceAddrs != nil {
		local, _ = r.env.InterfaceAddrs()
	}
	for _, ip := range ips {
		if ip.IsLoopback() || slices.ContainsFunc(local, ip.Equal) {
			r.add(HD5, Entry{Status: Warn, Code: CodeServerHostDetected})
			r.res.IsServerHost = true
			return
		}
	}
}

func (r *run) hd8Service() {
	for _, p := range []string{r.env.StateDir, r.env.KeyFile} {
		sec, err := r.env.Security.Lstat(p)
		if err != nil {
			continue
		}
		for _, s := range r.subjects {
			if sec.OwnerID == s.ID || hostos.Readable(sec, s) || hostos.Writable(sec, s) {
				r.fail(HD8, CodeStateAccessForSessionUser)
				return
			}
		}
	}
}

// trusted applies the rule of a place for all (HD10, HD11): after resolution the place, every
// folder on the way, every configured component and every link with its folder belong to a
// trusted owner and are writable by no session user. An unresolvable path is not trusted.
func (r *run) trusted(p string) bool {
	res, err := r.env.Resolver.Resolve(p)
	if err != nil {
		return false
	}
	objects := slices.Concat(res.Chain, res.Components, []string{res.Final})
	for _, l := range res.Links {
		objects = append(objects, l.Path, l.Dir)
	}
	for _, o := range objects {
		sec, err := r.env.Security.Lstat(o)
		if err != nil || !hostos.TrustedOwner(r.env.GOOS, sec) {
			return false
		}
		for _, s := range r.subjects {
			if hostos.Writable(sec, s) {
				return false
			}
		}
	}
	return true
}

// helperIntegrity checks the helper binary (also for setuid and setgid), every service
// definition, every endpoint folder and the configuration file (HD10).
func (r *run) helperIntegrity() {
	objects := slices.Concat([]string{r.env.HelperBinary, r.env.Config.Path}, r.env.ServiceDefinitions, r.env.EndpointDirs)
	untrusted := slices.ContainsFunc(objects, func(p string) bool { return !r.trusted(p) })
	if res, err := r.env.Resolver.Resolve(r.env.HelperBinary); err == nil {
		sec, err := r.env.Security.Lstat(res.Final)
		untrusted = untrusted || (err == nil && sec.Mode&(fs.ModeSetuid|fs.ModeSetgid) != 0)
	}
	if untrusted {
		r.fail(HD10, CodeHelperUntrusted)
	}
}

// shebangHead bounds the bytes read for the first line of host_service.claude_bin.
const shebangHead = 512

func (r *run) hd10(ctx context.Context) {
	r.helperIntegrity()
	r.claudeBin = r.checkClaudeBin()
	r.facts.claudeBinEmpty = r.claudeBin == ""
	if !r.env.PeerCheckProven {
		r.fail(HD10, CodePeerCheckUnavailable)
	}
	if r.env.GOOS == hostos.Linux {
		if scope, err := r.env.Probe.PtraceScope(); err != nil || scope < 1 {
			r.fail(HD10, CodePtraceScopeLow)
		}
	}
	if r.env.GOOS == hostos.Darwin && r.env.Codesign != nil {
		info, err := r.env.Codesign(ctx, r.env.HelperBinary)
		if err != nil || !info.Runtime || info.GetTaskAllow {
			r.fail(HD10, CodeHelperUnprotected)
		}
	}
}

// checkClaudeBin checks host_service.claude_bin, its folders, the search list and the
// interpreter (HD10, 24 7.4, D-W-85) and returns the resolved path HD6 and the profile use, or ""
// when it is empty or counts as empty.
func (r *run) checkClaudeBin() string {
	cb := r.env.Config.ClaudeBin
	if cb == "" {
		return ""
	}
	res, err := r.env.Resolver.Resolve(cb)
	if err != nil {
		return "" // unresolvable: counts as empty
	}
	ok := true
	if r.env.GOOS == hostos.Windows && !strings.EqualFold(filepath.Ext(res.Final), ".exe") {
		r.fail(HD10, CodeClaudeBinUntrusted)
		return ""
	}
	r.search = append([]string{filepath.Dir(filepath.Clean(cb))}, r.env.SystemSearchDirs...)
	if !r.trusted(cb) || !r.searchListTrusted() {
		// Never read an untrusted file: a session user could put a FIFO there.
		r.fail(HD10, CodeClaudeBinUntrusted)
		return ""
	}
	effective := res.Final
	head, err := r.env.Files.Head(res.Final, shebangHead)
	if err == nil && bytes.HasPrefix(head, []byte("#!")) {
		interp, iok := r.interpreter(head)
		ok = ok && iok
		if interp == "" {
			effective = ""
		}
	}
	if !ok {
		r.fail(HD10, CodeClaudeBinUntrusted)
		return "" // an untrusted program never runs in a session user
	}
	return effective
}

// searchListTrusted checks every folder of the search list that exists; a missing folder of the
// list finds nothing.
func (r *run) searchListTrusted() bool {
	for _, d := range r.search {
		if _, err := r.env.Resolver.Resolve(d); err == nil && !r.trusted(d) {
			return false
		}
	}
	return true
}

// interpreter checks the interpreter of a script; for "#!/usr/bin/env name" it checks env and
// the first name the search list finds. It returns the interpreter, "" when it is missing.
func (r *run) interpreter(head []byte) (string, bool) {
	line, _, _ := bytes.Cut(head[2:], []byte("\n"))
	f := strings.Fields(string(line))
	if len(f) == 0 {
		return "", true
	}
	if _, err := r.env.Resolver.Resolve(f[0]); err != nil {
		return "", true
	}
	ok := r.trusted(f[0])
	if filepath.Base(f[0]) != "env" {
		return f[0], ok
	}
	name := ""
	for _, a := range f[1:] {
		if !strings.HasPrefix(a, "-") {
			name = a
			break
		}
	}
	if name == "" {
		return "", ok
	}
	for _, d := range r.search {
		p := filepath.Join(d, name)
		if _, err := r.env.Resolver.Resolve(p); err == nil {
			iok := r.trusted(p)
			return p, ok && iok
		}
	}
	return "", ok
}

// hd7Profiles drops a command profile whose program is not absolute or cannot be resolved.
func (r *run) hd7Profiles() {
	for _, p := range r.env.Config.Profiles {
		if p.Kind != config.KindCommand {
			continue
		}
		if !filepath.IsAbs(p.Program) {
			r.facts.droppedProfile = append(r.facts.droppedProfile, p.Name)
			continue
		}
		if _, err := r.env.Resolver.Resolve(p.Program); err != nil {
			r.facts.droppedProfile = append(r.facts.droppedProfile, p.Name)
		}
	}
}

// placesForAll applies the rule of a place for all (HD11): the helper binary,
// host_service.claude_bin and every command program that can be resolved; on Windows also the
// root of every drive with a place.
func (r *run) placesForAll() {
	places := []string{r.env.HelperBinary}
	candidates := []string{r.env.Config.ClaudeBin}
	for _, p := range r.env.Config.Profiles {
		if p.Kind == config.KindCommand && filepath.IsAbs(p.Program) {
			candidates = append(candidates, p.Program)
		}
	}
	for _, p := range candidates {
		if _, err := r.env.Resolver.Resolve(p); p != "" && err == nil {
			places = append(places, p)
		}
	}
	untrusted := slices.ContainsFunc(places, func(p string) bool { return !r.trusted(p) })
	if r.env.GOOS == hostos.Windows {
		for _, d := range r.driveRoots(places) {
			if r.writableBySessionUser(d + `\`) {
				r.fail(HD11, CodeDriveRootWritable, "drive="+d)
			}
		}
	}
	if untrusted {
		r.fail(HD11, CodeProgramLocationUntrusted)
	}
}

// writableBySessionUser reports whether a session user can write p; a p that cannot be read
// counts as absent.
func (r *run) writableBySessionUser(p string) bool {
	sec, err := r.env.Security.Lstat(p)
	return err == nil && slices.ContainsFunc(r.subjects, func(s hostos.Subject) bool { return hostos.Writable(sec, s) })
}

// driveRoots returns the system drive and the drive of every configured place (Windows).
func (r *run) driveRoots(places []string) []string {
	all := slices.Clone(places)
	for _, l := range r.env.Config.ClaudeWorkdirs {
		for _, w := range l.Dirs {
			all = append(all, w.Path)
		}
	}
	roots := []string{strings.ToUpper(r.env.SystemDrive)}
	for _, p := range all {
		if v := strings.ToUpper(filepath.VolumeName(p)); len(v) == 2 && !slices.Contains(roots, v) {
			roots = append(roots, v)
		}
	}
	return slices.DeleteFunc(roots, func(s string) bool { return s == "" })
}

// helperAnswer is the answer of one helper.
type helperAnswer struct {
	user    string
	res     HelperResult
	err     error
	reached bool
}

// Request returns the check.run request for sessionUser.
func (r *run) request(u string) Request {
	mode := r.env.Mode
	if mode == "" {
		mode = ModeFull
	}
	var dirs []string
	for _, w := range r.env.Config.Workdirs(u) {
		dirs = append(dirs, w.Path)
	}
	return Request{
		Mode: mode, SessionUser: u, Others: slices.DeleteFunc(slices.Clone(r.users), func(s string) bool { return s == u }),
		ServiceUser: r.env.ServiceUser, ClaudeBin: r.claudeBin, SearchList: r.search, Declaration: r.env.Declaration,
		Workdirs: dirs, ForeignHomes: r.foreignHomes(), StateDir: r.env.StateDir, KeyFile: r.env.KeyFile,
		AuthStatusTimeout: r.env.Config.AuthStatusTimeout, EntryScanTimeout: r.env.Config.EntryScanTimeout,
	}
}

// foreignHomes returns the homes HD9 lists: root, the service user, every former session user
// and every person account, each only when it is not a session user (D-M2-42-6).
func (r *run) foreignHomes() []NamedHome {
	accs, err := r.env.Users.Accounts()
	if err != nil {
		return nil
	}
	r.former = r.former[:0]
	for name := range r.env.Bindings {
		if !slices.Contains(r.env.Config.SessionUsers, name) {
			r.former = append(r.former, name)
		}
	}
	slices.Sort(r.former)
	var out []NamedHome
	for _, a := range accs {
		if slices.Contains(r.env.Config.SessionUsers, a.Name) || a.Home == "" {
			continue
		}
		if a.Name == "root" || a.Name == r.env.ServiceUser || slices.Contains(r.former, a.Name) || a.Person {
			out = append(out, NamedHome{Name: a.Name, Home: a.Home})
		}
	}
	return out
}

func (r *run) askHelpers(ctx context.Context) []helperAnswer {
	answers := make([]helperAnswer, len(r.users))
	r.res.requests = map[string]Request{}
	var wg sync.WaitGroup
	for i, u := range r.users {
		req := r.request(u)
		r.res.requests[u] = req
		wg.Go(func() {
			cctx, cancel := context.WithTimeout(ctx, r.env.checkBound())
			defer cancel()
			res, err := r.env.Helpers.Check(cctx, u, req)
			answers[i] = helperAnswer{user: u, res: res, err: err}
			if err == nil {
				answers[i].reached = true
			}
		})
	}
	wg.Wait()
	return answers
}

func (r *run) aggregate(answers []helperAnswer) {
	if ok, err := r.env.Probe.SudoWithoutPassword(context.Background()); r.env.GOOS != hostos.Windows && (ok || err != nil) {
		r.fail(HD3, CodePasswordlessSudo)
	}
	r.facts.llm = map[string]LLMFacts{}
	ids := map[string][]string{} // identity -> session users
	idOf := map[string][]string{}
	listable := map[string]bool{}
	for _, a := range answers {
		if a.err != nil {
			code := CodeHelperUnreachable
			if errors.Is(a.err, ErrPeerMismatch) {
				code = CodePeerMismatch
			}
			r.add(HD10, Entry{Status: Fail, Code: code, SessionUser: a.user})
			continue
		}
		r.takeHelper(a.user, a.res, listable)
		idOf[a.user] = a.res.WorkdirIDs
		for _, id := range slices.Compact(slices.Sorted(slices.Values(slices.DeleteFunc(slices.Clone(a.res.WorkdirIDs), func(s string) bool { return s == "" })))) {
			ids[id] = append(ids[id], a.user)
		}
	}
	r.hd9(listable)
	r.hd7Workdirs(idOf, ids)
	r.workdirPlaces()
	r.facts.profiles = r.env.Config.Profiles
	r.facts.sessionUsers = r.env.Config.SessionUsers
}

// takeHelper adds the answer of the helper of user to HD3, HD6, HD8, HD9 and HD11.
func (r *run) takeHelper(user string, res HelperResult, listable map[string]bool) {
	r.res.reached = append(r.res.reached, user)
	if res.Sudo && r.env.GOOS != hostos.Windows {
		r.fail(HD3, CodePasswordlessSudo)
	}
	r.facts.llm[user] = res.LLM
	if res.ReadsState {
		r.fail(HD8, CodeHelperReadsState)
	}
	for _, n := range res.Listable {
		listable[n] = true
	}
	if res.PlaceCode != "" {
		r.add(HD11, Entry{Status: Fail, Code: res.PlaceCode, SessionUser: user})
	}
	if r.workdirCodes == nil {
		r.workdirCodes = map[string]map[string]string{}
	}
	codes := map[string]string{}
	for i, w := range r.env.Config.Workdirs(user) {
		if i < len(res.WorkdirCodes) && res.WorkdirCodes[i] != "" {
			codes[w.Path] = res.WorkdirCodes[i]
		}
	}
	r.workdirCodes[user] = codes
}

// workdirPlaces adds HD11 for the working directories HD7 keeps; a directory HD7 drops is no place
// of the session user (M2-45, TM-T29 (c)).
func (r *run) workdirPlaces() {
	for _, u := range r.env.Config.SessionUsers {
		for _, w := range r.res.Workdirs[u] {
			if code := r.workdirCodes[u][w.Path]; code != "" {
				r.add(HD11, Entry{Status: Fail, Code: code, SessionUser: u})
			}
		}
	}
}

func (r *run) hd9(listable map[string]bool) {
	names := make([]string, 0, len(listable))
	for n := range listable {
		names = append(names, n)
	}
	slices.Sort(names)
	var unnamed []string
	for _, n := range names {
		if slices.Contains(r.former, n) {
			r.add(HD9, Entry{Status: Fail, Code: CodeForeignHomeListable, SessionUser: n, Local: []string{"user=" + n}})
			continue
		}
		unnamed = append(unnamed, "user="+n)
	}
	if len(unnamed) > 0 {
		r.add(HD9, Entry{Status: Fail, Code: CodeForeignHomeListable, Local: unnamed})
	}
}

// hd7Workdirs builds the lists after HD7 (24 2.2 point 6): a key that is no session user drops
// with its list, a folder its helper cannot resolve drops, a folder under two session users drops
// from both lists, and an empty or repeated display name drops.
func (r *run) hd7Workdirs(idOf map[string][]string, ids map[string][]string) {
	var dropped []string
	for _, l := range r.env.Config.ClaudeWorkdirs {
		if !slices.Contains(r.env.Config.SessionUsers, l.SessionUser) {
			for _, w := range l.Dirs {
				dropped = append(dropped, "item="+w.DisplayName)
			}
		}
	}
	for _, u := range r.env.Config.SessionUsers {
		list, gone := r.workdirList(u, idOf[u], ids)
		r.res.Workdirs[u] = list
		dropped = append(dropped, gone...)
	}
	if len(dropped) > 0 {
		r.add(HD7, Entry{Status: Warn, Code: CodeWorkdirDropped, Local: dropped})
	}
}

// workdirList returns the list of u after HD7 and the local names of the dropped items.
func (r *run) workdirList(u string, got []string, ids map[string][]string) ([]Workdir, []string) {
	list := []Workdir{}
	var dropped []string
	seen := map[string]bool{}
	for i, w := range r.env.Config.Workdirs(u) {
		id := ""
		if i < len(got) {
			id = got[i]
		}
		if w.DisplayName == "" || seen[w.DisplayName] || id == "" || len(ids[id]) > 1 {
			dropped = append(dropped, "item="+w.DisplayName)
			continue
		}
		seen[w.DisplayName] = true
		list = append(list, Workdir{DisplayName: w.DisplayName, Path: w.Path})
	}
	return list, dropped
}

// evaluate (re)computes HD6, the profile part of HD7 and the lists with the declaration decl;
// a new declaration from ready changes the result without a new run (D-M2-42-15).
func (res *Result) evaluate(decl string) {
	res.Declaration = decl
	f := res.facts
	hd6 := CheckResult{Check: HD6}
	unchecked := false
	users := make([]string, 0, len(f.llm))
	for u := range f.llm {
		users = append(users, u)
	}
	slices.Sort(users)
	for _, u := range users {
		l := f.llm[u]
		unchecked = unchecked || l.Unchecked
		code := ""
		switch {
		case l.Finding:
			code = CodeSubscriptionLogin
		case l.Unknown:
			code = CodeAuthStatusUnknown
		}
		switch {
		case code == "" || decl == state.DeclPersonal:
		case decl == state.DeclUnknown:
			hd6.Entries = append(hd6.Entries, Entry{Status: Warn, Code: CodeDeclarationUnknown, SessionUser: u})
		default:
			hd6.Entries = append(hd6.Entries, Entry{Status: Fail, Code: code, SessionUser: u})
		}
	}
	if unchecked {
		hd6.Entries = append(hd6.Entries, Entry{Status: Warn, Code: CodeAuthStatusUnchecked})
	}
	res.set(hd6)
	res.evalProfiles(decl)
}

func (res *Result) evalProfiles(decl string) {
	f := res.facts
	hd7 := res.Check(HD7)
	hd7.Entries = slices.DeleteFunc(hd7.Entries, func(e Entry) bool { return e.Code != CodeWorkdirDropped })
	res.Profiles = []Profile{}
	for _, p := range f.profiles {
		switch p.Kind {
		case config.KindClaude:
			switch {
			case decl == state.DeclNone:
				continue
			case decl != state.DeclPersonal && f.claudeBinEmpty:
				hd7.Entries = append(hd7.Entries, Entry{Status: Warn, Code: CodeClaudeProfileUnavailable, Local: []string{"item=" + p.Name}})
				continue
			}
		case config.KindCommand:
			if slices.Contains(f.droppedProfile, p.Name) {
				hd7.Entries = append(hd7.Entries, Entry{Status: Warn, Code: CodeProfileDropped, Local: []string{"item=" + p.Name}})
				continue
			}
		}
		res.Profiles = append(res.Profiles, Profile{Name: p.Name, DisplayName: p.DisplayName, Kind: p.Kind})
	}
	res.set(collapseHD7(hd7))
	res.SessionUsers = []ReportedSessionUser{}
	for _, u := range f.sessionUsers {
		names := []string{}
		for _, w := range res.Workdirs[u] {
			names = append(names, w.DisplayName)
		}
		res.SessionUsers = append(res.SessionUsers, ReportedSessionUser{OSUser: u, Workdirs: names})
	}
}

// collapseHD7 merges the warnings of HD7 into one entry with the code by precedence and every
// item in the local output.
func collapseHD7(c CheckResult) CheckResult {
	if len(c.Entries) <= 1 {
		return c
	}
	e := Entry{Status: Warn, Code: c.Entries[0].Code}
	for _, x := range c.Entries {
		if rank(HD7, x.Code) < rank(HD7, e.Code) {
			e.Code = x.Code
		}
		e.Local = append(e.Local, x.Local...)
	}
	return CheckResult{Check: HD7, Entries: []Entry{e}}
}

// WithDeclaration returns a copy of r evaluated with decl (a ready with a declaration).
func (res *Result) WithDeclaration(decl string) *Result {
	c := *res
	c.Checks = slices.Clone(res.Checks)
	c.evaluate(decl)
	return &c
}

// ParseCodesign reads the hardened runtime flag from the output of "codesign --display
// --verbose=4" (the line "CodeDirectory ... flags=0x10000(runtime)") and get-task-allow from the
// entitlements of "codesign --display --entitlements - --xml".
func ParseCodesign(display, entitlements []byte) CodesignInfo {
	var info CodesignInfo
	for _, line := range strings.Split(string(display), "\n") {
		if strings.HasPrefix(line, "CodeDirectory") {
			if _, flags, ok := strings.Cut(line, "flags="); ok {
				f, _, _ := strings.Cut(flags, " ")
				info.Runtime = strings.Contains(f, "runtime")
			}
		}
	}
	ent := string(entitlements)
	if i := strings.Index(ent, "com.apple.security.get-task-allow"); i >= 0 {
		rest := strings.TrimSpace(ent[i+len("com.apple.security.get-task-allow"):])
		rest = strings.TrimPrefix(rest, "</key>")
		info.GetTaskAllow = strings.HasPrefix(strings.TrimSpace(rest), "<true/>")
	}
	return info
}
