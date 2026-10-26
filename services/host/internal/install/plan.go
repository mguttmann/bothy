// SPDX-License-Identifier: Apache-2.0

// Package install computes and executes the plans of "bothy-host install", "install
// --drive-root", "session-user add", "session-user remove" and "restart" (docs/spec/24-terminal.md
// 2.2, docs/spec/16-operations.md 4.4, docs/DECISIONS.md D-M2-42-13 and -14). A plan is a list of
// steps with argument vectors; every step is idempotent, and nothing existing is removed or
// overwritten except by session-user remove. Commands start by absolute path from /usr/bin, /bin,
// /usr/sbin and /sbin (Windows: the service control manager and system APIs), never through a
// shell or PATH. The service definitions are rendered from the templates in units/.
package install

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

// Step kinds.
const (
	KindRun          = "run"           // Argv
	KindRunTolerant  = "run?"          // Argv; its failure is ignored (unloading what may not be loaded)
	KindMkdir        = "mkdir"         // Path, Mode, Owner
	KindWrite        = "write"         // Path, Mode, Owner, Content
	KindCopy         = "copy"          // Source -> Path, Mode, Owner
	KindChmod        = "chmod"         // Path, Mode
	KindRemove       = "remove"        // Path
	KindConfigAdd    = "config.add"    // Path, Name
	KindConfigRemove = "config.remove" // Path, Name
	// Windows steps, applied through system APIs by the executor of Windows.
	KindWinUser       = "win.user"        // Name: local account with a random password kept for the service
	KindWinProfile    = "win.profile"     // Name: CreateProfile and an ACL only for the user and the trusted owners
	KindWinService    = "win.service"     // Name, Content (rendered definition)
	KindWinServiceDel = "win.service.del" // Name
	KindWinRestart    = "win.restart"     // Name: stop when running, then start
	KindWinACL        = "win.acl"         // Path, Content (SDDL of a protected DACL)
	KindWinDriveRoot  = "win.driveroot"   // Path: remove Authenticated Users:(AD) with SetFileSecurityW
)

// Step is one idempotent action of a plan.
type Step struct {
	Kind    string
	Argv    []string
	Path    string
	Source  string
	Name    string
	Owner   string
	Mode    fs.FileMode
	Content string
}

// String renders the step for golden tests and the dry output; the content of a definition is
// named by its length only.
func (s Step) String() string {
	switch s.Kind {
	case KindRun, KindRunTolerant:
		return s.Kind + " " + strings.Join(s.Argv, " ")
	case KindMkdir, KindChmod:
		return fmt.Sprintf("%s %s %04o %s", s.Kind, s.Path, uint32(s.Mode.Perm()), s.Owner)
	case KindWrite:
		return fmt.Sprintf("write %s %04o %s (%d bytes)", s.Path, uint32(s.Mode.Perm()), s.Owner, len(s.Content))
	case KindCopy:
		return fmt.Sprintf("copy %s -> %s %04o %s", s.Source, s.Path, uint32(s.Mode.Perm()), s.Owner)
	case KindRemove, KindWinDriveRoot:
		return s.Kind + " " + s.Path
	case KindWinACL:
		return s.Kind + " " + s.Path + " " + s.Content
	case KindWinService:
		return fmt.Sprintf("%s %s (%d bytes)", s.Kind, s.Name, len(s.Content))
	}
	return strings.Join(slices.DeleteFunc([]string{s.Kind, s.Path, s.Name}, func(x string) bool { return x == "" }), " ")
}

//go:embed units/*.tmpl
var units embed.FS

var tmpl = template.Must(template.ParseFS(units, "units/*.tmpl"))

func render(name string, data any) string {
	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, name, data); err != nil {
		panic(err) // the templates are embedded and covered by the golden tests
	}
	return b.String()
}

// Owners named in the steps.
const (
	ownerRoot   = "root"
	ownerAdmins = "Administrators"
	hostLabel   = "bothy-host"
)

// Paths of the product name per platform (02 "Places that carry the product name").
const (
	LinuxProgramDir  = "/opt/bothy-host/bin"
	DarwinProgramDir = "/Library/Application Support/bothy-host/bin"
	// DarwinConfirmDir holds the socket of the local confirmation (M2-45, internal/confirm).
	DarwinConfirmDir = "/Library/Application Support/bothy-host/confirm"
	SystemdDir       = "/etc/systemd/system"
	LaunchDaemonsDir = "/Library/LaunchDaemons"
	// HostAccountWindows is the virtual account of the host service on Windows.
	HostAccountWindows = `NT SERVICE\bothy-host`
)

// Target describes the host a plan is for.
type Target struct {
	GOOS        string
	ServiceUser string
	// StateDir is host_service.state_dir; ConfigPath the configuration file.
	StateDir, ConfigPath string
	// Running is the path of the running binary that install copies.
	Running string
	// ProgramFiles is %ProgramFiles% on Windows.
	ProgramFiles string
	// Users is the user database (users that exist are not created again).
	Users hostos.Users
	// SignedWithRuntime tells that the binary already carries a signature with the runtime flag
	// (macOS, the Developer ID signature of M7-23), so install does not sign it again.
	SignedWithRuntime bool
	// ConfigExists tells that the configuration file exists (install never overwrites it).
	ConfigExists bool
	// HostRunning tells that the host service runs (Windows restart).
	HostRunning bool
}

// Binary returns the installed program path.
func (t Target) Binary() string {
	switch t.GOOS {
	case hostos.Darwin:
		return filepath.Join(DarwinProgramDir, "bothy-host")
	case hostos.Windows:
		return t.ProgramFiles + `\bothy-host\bothy-host.exe`
	}
	return filepath.Join(LinuxProgramDir, "bothy-host")
}

// ServiceDefinitions returns the files of the service definitions of the host and the helpers
// (HD10 checks them; Windows keeps them in the service control manager).
func (t Target) ServiceDefinitions(sessionUsers []string) []string {
	var out []string
	switch t.GOOS {
	case hostos.Linux:
		out = append(out, filepath.Join(SystemdDir, "bothy-host.service"))
		for _, u := range sessionUsers {
			out = append(out, filepath.Join(SystemdDir, "bothy-host-helper-"+u+".socket"), filepath.Join(SystemdDir, hostos.HelperServiceName(hostos.Linux, u)))
		}
	case hostos.Darwin:
		out = append(out, filepath.Join(LaunchDaemonsDir, "bothy-host.plist"))
		for _, u := range sessionUsers {
			out = append(out, filepath.Join(LaunchDaemonsDir, hostos.HelperServiceName(hostos.Darwin, u)+".plist"))
		}
	}
	return out
}

func (t Target) exists(name string) bool {
	if t.Users == nil {
		return false
	}
	_, err := t.Users.Lookup(name)
	return err == nil
}

// ErrUnsupported is returned for a platform without a plan.
var ErrUnsupported = errors.New("install: unsupported platform")

// hostUnit returns the path and the content of the service definition of the host service on
// Linux (systemd) and macOS (launchd).
func hostUnit(t Target) (path, content string) {
	if t.GOOS == hostos.Darwin {
		return filepath.Join(LaunchDaemonsDir, "bothy-host.plist"), render("host.plist.tmpl",
			map[string]string{"Label": hostLabel, keyServiceUser: t.ServiceUser, keyBinary: t.Binary(), "Config": t.ConfigPath})
	}
	return filepath.Join(SystemdDir, "bothy-host.service"), render("host.service.tmpl",
		map[string]string{keyServiceUser: t.ServiceUser, keyBinary: t.Binary(), "Config": t.ConfigPath})
}

// HostDefinition returns the path and the content of the service definition of the host service
// that bothy-host install writes on goos ("linux": the systemd unit, "darwin": the LaunchDaemon)
// for the service user serviceUser and the configuration file configPath. The .deb, .rpm and .pkg
// packages carry exactly this file as a reference in the share folder next to the binary
// (infra/host, M7-23); install writes the live definition, which a package upgrade never touches.
//
// Errors: ErrUnsupported for "windows", whose MSI registers the service itself, and any other
// platform.
//
// Example: HostDefinition("linux", "bothy-host", "/etc/bothy-host/config.yaml") returns
// "/etc/systemd/system/bothy-host.service" and the unit.
func HostDefinition(goos, serviceUser, configPath string) (path, content string, err error) {
	if goos != hostos.Linux && goos != hostos.Darwin {
		return "", "", ErrUnsupported
	}
	path, content = hostUnit(Target{GOOS: goos, ServiceUser: serviceUser, ConfigPath: configPath})
	return path, content, nil
}

// Install returns the plan of "bothy-host install" with the session users of the configuration
// plus extra (--session-user).
func Install(t Target, sessionUsers []string) ([]Step, error) {
	switch t.GOOS {
	case hostos.Linux:
		return linuxInstall(t, sessionUsers), nil
	case hostos.Darwin:
		return darwinInstall(t, sessionUsers)
	case hostos.Windows:
		return windowsInstall(t, sessionUsers), nil
	}
	return nil, ErrUnsupported
}

// Template fields used by several templates.
const (
	keyBinary      = "Binary"
	keyName        = "Name"
	keyServiceUser = "ServiceUser"
)

func run(argv ...string) Step { return Step{Kind: KindRun, Argv: argv} }

// load bootstraps a LaunchDaemon; a label loaded by an earlier install is unloaded first, so a
// repeated install does not fail.
func load(label, plist string) []Step {
	return []Step{{Kind: KindRunTolerant, Argv: []string{"/bin/launchctl", "bootout", "system/" + label}},
		run("/bin/launchctl", "bootstrap", "system", plist)}
}

func configStep(t Target, users []string) []Step {
	if t.ConfigExists {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Configuration of the host service (docs/spec/24-terminal.md section 17).\nhost_service:\n  session_users: [")
	b.WriteString(strings.Join(users, ", "))
	b.WriteString("]\n")
	owner := ownerRoot
	if t.GOOS == hostos.Windows {
		owner = ownerAdmins
	}
	return []Step{{Kind: KindWrite, Path: t.ConfigPath, Mode: 0o644, Owner: owner, Content: b.String()}}
}

// linuxHelper returns the steps of one session user and its helper on Linux.
func linuxHelper(t Target, u string) []Step {
	var steps []Step
	if !t.exists(u) {
		steps = append(steps, run("/usr/sbin/useradd", "--create-home", "--user-group", "--shell", "/bin/bash", u))
	}
	socket := "bothy-host-helper-" + u + ".socket"
	service := hostos.HelperServiceName(hostos.Linux, u)
	data := map[string]string{keyName: u, keyServiceUser: t.ServiceUser, keyBinary: t.Binary(), "Endpoint": localchan.HelperEndpoint(hostos.Linux, u), "Socket": socket}
	return append(steps,
		Step{Kind: KindChmod, Path: "~" + u, Mode: 0o700, Owner: u},
		Step{Kind: KindWrite, Path: filepath.Join(SystemdDir, socket), Mode: 0o644, Owner: ownerRoot, Content: render("helper.socket.tmpl", data)},
		Step{Kind: KindWrite, Path: filepath.Join(SystemdDir, service), Mode: 0o644, Owner: ownerRoot, Content: render("helper.service.tmpl", data)},
	)
}

func linuxInstall(t Target, users []string) []Step {
	var steps []Step
	if !t.exists(t.ServiceUser) {
		steps = append(steps, run("/usr/sbin/useradd", "--system", "--user-group", "--home-dir", t.StateDir, "--no-create-home", "--shell", "/usr/sbin/nologin", t.ServiceUser))
	}
	steps = append(steps,
		Step{Kind: KindMkdir, Path: t.StateDir, Mode: 0o700, Owner: t.ServiceUser},
		Step{Kind: KindMkdir, Path: LinuxProgramDir, Mode: 0o755, Owner: ownerRoot},
		Step{Kind: KindCopy, Source: t.Running, Path: t.Binary(), Mode: 0o755, Owner: ownerRoot},
		Step{Kind: KindMkdir, Path: filepath.Dir(t.ConfigPath), Mode: 0o755, Owner: ownerRoot},
	)
	steps = append(steps, configStep(t, users)...)
	for _, u := range users {
		steps = append(steps, linuxHelper(t, u)...)
	}
	unit, content := hostUnit(t)
	steps = append(steps, Step{Kind: KindWrite, Path: unit, Mode: 0o644, Owner: ownerRoot, Content: content})
	steps = append(steps, run("/usr/bin/systemctl", "daemon-reload"))
	for _, u := range users {
		steps = append(steps, run("/usr/bin/systemctl", "enable", "--now", "bothy-host-helper-"+u+".socket"))
	}
	return append(steps, run("/usr/bin/systemctl", "enable", "bothy-host.service"), run("/usr/bin/systemctl", "restart", "bothy-host.service"))
}

// darwinIDs picks free IDs: the service user gets the first free UniqueID from 400 (hidden system
// range), each session user the first free one from 501.
func darwinIDs(t Target, users []string) (svc int, session map[string]int, err error) {
	taken := map[int]bool{}
	ids := map[string]int{}
	if t.Users != nil {
		accs, aerr := t.Users.Accounts()
		if aerr != nil {
			return 0, nil, fmt.Errorf("install: %w", aerr)
		}
		for _, a := range accs {
			n, _ := strconv.Atoi(a.ID)
			taken[n] = true
			ids[a.Name] = n
		}
	}
	next := func(from int) int {
		for i := from; ; i++ {
			if !taken[i] {
				taken[i] = true
				return i
			}
		}
	}
	svc, ok := ids[t.ServiceUser]
	if !ok {
		svc = next(400)
	}
	session = map[string]int{}
	for _, u := range users {
		if id, ok := ids[u]; ok {
			session[u] = id
			continue
		}
		session[u] = next(501)
	}
	return svc, session, nil
}

func dscl(args ...string) Step { return run(append([]string{"/usr/bin/dscl", "."}, args...)...) }

func darwinUser(name string, id int, home, shell string) []Step {
	ids := strconv.Itoa(id)
	return []Step{
		dscl("-create", "/Groups/"+name),
		dscl("-create", "/Groups/"+name, "PrimaryGroupID", ids),
		dscl("-create", "/Users/"+name),
		dscl("-create", "/Users/"+name, "UniqueID", ids),
		dscl("-create", "/Users/"+name, "PrimaryGroupID", ids),
		dscl("-create", "/Users/"+name, "UserShell", shell),
		dscl("-create", "/Users/"+name, "NFSHomeDirectory", home),
		dscl("-create", "/Users/"+name, "IsHidden", "1"),
	}
}

func darwinHelper(t Target, u string, id, svcGID int) []Step {
	var steps []Step
	if !t.exists(u) {
		steps = append(steps, darwinUser(u, id, "/Users/"+u, "/bin/zsh")...)
		steps = append(steps, run("/usr/sbin/createhomedir", "-c", "-u", u))
	}
	label := hostos.HelperServiceName(hostos.Darwin, u)
	data := map[string]string{keyName: u, "Label": label, keyBinary: t.Binary(), "Endpoint": localchan.HelperEndpoint(hostos.Darwin, u), "ServiceGID": strconv.Itoa(svcGID)}
	return append(steps,
		Step{Kind: KindChmod, Path: "~" + u, Mode: 0o700, Owner: u},
		Step{Kind: KindWrite, Path: filepath.Join(LaunchDaemonsDir, label+".plist"), Mode: 0o644, Owner: ownerRoot, Content: render("helper.plist.tmpl", data)},
	)
}

func darwinInstall(t Target, users []string) ([]Step, error) {
	svc, ids, err := darwinIDs(t, users)
	if err != nil {
		return nil, err
	}
	var steps []Step
	if !t.exists(t.ServiceUser) {
		steps = append(steps, darwinUser(t.ServiceUser, svc, t.StateDir, "/usr/bin/false")...)
	}
	steps = append(steps,
		Step{Kind: KindMkdir, Path: t.StateDir, Mode: 0o700, Owner: t.ServiceUser},
		Step{Kind: KindMkdir, Path: DarwinProgramDir, Mode: 0o755, Owner: ownerRoot},
		Step{Kind: KindCopy, Source: t.Running, Path: t.Binary(), Mode: 0o755, Owner: ownerRoot},
	)
	if !t.SignedWithRuntime {
		steps = append(steps, run("/usr/bin/codesign", "--force", "--sign", "-", "--options", "runtime", t.Binary()))
	}
	steps = append(steps,
		Step{Kind: KindMkdir, Path: localchan.DarwinRunDir, Mode: 0o755, Owner: ownerRoot},
		// The folder of the confirmation endpoint (M2-45, 24 6.4), owned by the service user.
		Step{Kind: KindMkdir, Path: DarwinConfirmDir, Mode: 0o755, Owner: t.ServiceUser},
		Step{Kind: KindMkdir, Path: filepath.Dir(t.ConfigPath), Mode: 0o755, Owner: ownerRoot},
	)
	steps = append(steps, configStep(t, users)...)
	for _, u := range users {
		steps = append(steps, darwinHelper(t, u, ids[u], svc)...)
	}
	host, content := hostUnit(t)
	steps = append(steps, Step{Kind: KindWrite, Path: host, Mode: 0o644, Owner: ownerRoot, Content: content})
	for _, u := range users {
		label := hostos.HelperServiceName(hostos.Darwin, u)
		steps = append(steps, load(label, filepath.Join(LaunchDaemonsDir, label+".plist"))...)
	}
	steps = append(steps, load(hostLabel, host)...)
	return append(steps, run("/bin/launchctl", "kickstart", "-k", "system/bothy-host")), nil
}

// Windows SDDL of the state directory and of the helper services (D-M2-42-13, -14).
const (
	stateSDDL = "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)"
	// helperServiceSDDL lets SYSTEM and Administrators control the service; authenticated users
	// (the session user included) may only query it, never stop or reconfigure it.
	helperServiceSDDL = "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLORC;;;AU)"
	hostServiceSDDL   = "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLORC;;;AU)"
	// ServiceSIDPlaceholder stands for the SID of the virtual account, known only after the
	// service exists; the executor replaces it.
	ServiceSIDPlaceholder = "<service-sid>"
)

func windowsService(name, display, binary, account, sddl string) Step {
	return Step{Kind: KindWinService, Name: name, Content: render("windows-service.tmpl", map[string]string{
		"Name": name, "Display": display, "BinaryCommand": binary, "Account": account, "SDDL": sddl})}
}

// windowsHelper creates the account (or resets its password, which only the service control
// manager receives) and the helper service of u.
func windowsHelper(t Target, u string) []Step {
	steps := []Step{{Kind: KindWinUser, Name: u}}
	return append(steps,
		Step{Kind: KindWinProfile, Name: u},
		windowsService(hostos.HelperServiceName(hostos.Windows, u), "bothy-host PTY helper of "+u,
			`"`+t.Binary()+`" pty-helper --session-user `+u, `.\`+u, helperServiceSDDL),
	)
}

func windowsInstall(t Target, users []string) []Step {
	dir := t.ProgramFiles + `\bothy-host`
	steps := []Step{
		windowsService(hostLabel, "bothy-host service", `"`+t.Binary()+`" run --config "`+t.ConfigPath+`"`, HostAccountWindows, hostServiceSDDL),
		{Kind: KindMkdir, Path: t.StateDir, Mode: 0o700, Owner: HostAccountWindows},
		{Kind: KindWinACL, Path: t.StateDir, Content: fmt.Sprintf(stateSDDL, ServiceSIDPlaceholder)},
		{Kind: KindMkdir, Path: dir, Mode: 0o755, Owner: ownerAdmins},
		{Kind: KindCopy, Source: t.Running, Path: t.Binary(), Mode: 0o755, Owner: ownerAdmins},
	}
	steps = append(steps, configStep(t, users)...)
	for _, u := range users {
		steps = append(steps, windowsHelper(t, u)...)
	}
	return append(steps, Step{Kind: KindWinRestart, Name: hostLabel})
}

// DriveRoot returns the step of "install --drive-root <drive>" (Windows): remove only the entry
// Authenticated Users:(AD) at the root of drive. Without the operator's confirmation the plan is
// empty and HD11 keeps reporting drive_root_writable.
func DriveRoot(drive string, confirmed bool) []Step {
	if !confirmed {
		return nil
	}
	return []Step{{Kind: KindWinDriveRoot, Path: strings.ToUpper(strings.TrimSuffix(drive, `\`)) + `\`}}
}

// Restart returns the plan of "bothy-host restart": only the host service, also after a refused
// start (Linux resets the failed state first).
func Restart(t Target) ([]Step, error) {
	switch t.GOOS {
	case hostos.Linux:
		return []Step{run("/usr/bin/systemctl", "reset-failed", "bothy-host.service"), run("/usr/bin/systemctl", "restart", "bothy-host.service")}, nil
	case hostos.Darwin:
		return []Step{run("/bin/launchctl", "kickstart", "-k", "system/bothy-host")}, nil
	case hostos.Windows:
		return []Step{{Kind: KindWinRestart, Name: hostLabel}}, nil
	}
	return nil, ErrUnsupported
}
