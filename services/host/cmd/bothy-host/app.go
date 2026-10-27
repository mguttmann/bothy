// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// interfaceAddrs returns the addresses of the own interfaces (HD5).
func interfaceAddrs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var out []net.IP
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			out = append(out, n.IP)
		}
	}
	return out, nil
}

// codesignInfo reads the flags and entitlements of a macOS binary with codesign.
func codesignInfo(ctx context.Context, r hostos.Runner, binary string) (doctor.CodesignInfo, error) {
	disp, err := r.Run(ctx, hostos.Cmd{Path: "/usr/bin/codesign", Args: []string{"--display", "--verbose=4", binary}})
	if err != nil || disp.ExitCode != 0 {
		return doctor.CodesignInfo{}, errors.New("codesign failed")
	}
	ent, err := r.Run(ctx, hostos.Cmd{Path: "/usr/bin/codesign", Args: []string{"--display", "--entitlements", "-", "--xml", binary}})
	if err != nil || ent.ExitCode != 0 {
		return doctor.CodesignInfo{}, errors.New("codesign failed")
	}
	// codesign prints its display on standard error.
	return doctor.ParseCodesign(append(disp.Stdout, disp.Stderr...), ent.Stdout), nil
}

// Exit codes of the commands (D-M2-42-2).
const (
	exitFailed  = 1
	exitRefused = startup.ExitStartRefused
)

// serviceUser is the name of the service user on Linux and macOS (on Windows the virtual account
// NT SERVICE\bothy-host).
const serviceUser = "bothy-host"

// app is the operating system as the commands see it. Only this package reads the environment;
// the tests replace the functions.
type app struct {
	goos   string
	getenv func(string) string
	// isAdmin tells that the caller is root or an elevated member of Administrators.
	isAdmin func() bool
	// callerID is the uid (or SID) of the caller.
	callerID   func() string
	executable func() (string, error)
	stdin      io.Reader
	// interactive tells that standard input is a terminal (the prompt of the drive root).
	interactive func() bool
	runner      hostos.Runner
	platform    func(hostos.Runner) (hostos.Users, hostos.Security, hostos.Probe)
	resolver    hostos.Resolver
	files       hostos.Lister
	services    hostos.ServiceManager
	apply       func(ctx context.Context, steps []install.Step) error
	// dialAdmin connects to the admin endpoint of the running service.
	dialAdmin func(ctx context.Context, stateDir, serviceUID string) (*localchan.Conn, error)
	// asServiceUser runs this program with args as the service user (Linux and macOS: a child
	// process with its credentials) and returns its exit code.
	asServiceUser func(ctx context.Context, uid, gid int, args []string, stdin []byte, stdout, stderr io.Writer) (int, error)
	listenAdmin   func(path string) (net.Listener, error)
	random        io.Reader
	protector     state.Protector
	osVersion     func() string
	log           func(w io.Writer) *slog.Logger
}

var sys = realApp()

func realApp() *app {
	r := hostos.ExecRunner{}
	a := &app{
		goos: runtime.GOOS, getenv: os.Getenv, isAdmin: isAdmin, callerID: callerID, executable: os.Executable,
		stdin: os.Stdin, interactive: interactive, runner: r, platform: hostos.Platform, services: serviceManager(r),
		resolver: hostos.OSResolver{}, files: hostos.OSLister{},
		dialAdmin: func(ctx context.Context, stateDir, serviceUID string) (*localchan.Conn, error) {
			return localchan.DialAdmin(ctx, filepath.Join(stateDir, state.AdminSocket), serviceUID)
		},
		asServiceUser: asServiceUser, listenAdmin: localchan.ListenAdmin, random: rand.Reader, protector: state.PlatformProtector(),
		osVersion: osVersion,
		log:       func(w io.Writer) *slog.Logger { return slog.New(slog.NewJSONHandler(w, nil)) },
	}
	a.apply = func(ctx context.Context, steps []install.Step) error {
		users, _, _ := a.platform(a.runner)
		return install.Apply(ctx, installOps(a.runner, users), steps)
	}
	return a
}

func (a *app) cfgPlatform() config.Platform {
	return config.Platform{GOOS: a.goos, ProgramData: a.getenv("ProgramData"), ProgramFiles: a.getenv("ProgramFiles")}
}

// errUsage marks an error that exits 2.
var errUsage = errors.New("usage")

// loadConfig reads the configuration of --config or the default path; any error exits 2.
func (a *app) loadConfig(path string) (*config.Config, error) {
	if path == "" {
		path = config.DefaultPath(a.cfgPlatform())
	}
	c, err := config.Load(path, a.cfgPlatform())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUsage, err)
	}
	return c, nil
}

// adminRequired prints the message of D-M2-42-1 and returns 2.
func adminRequired(stderr io.Writer, cmd string) int {
	_, _ = fmt.Fprintf(stderr, "bothy-host %s: administrator rights required\n", cmd)
	return exitUsage
}

// target returns the install target of the configuration c.
func (a *app) target(c *config.Config, configExists bool) (install.Target, error) {
	exe, err := a.executable()
	if err != nil {
		return install.Target{}, err
	}
	users, _, _ := a.platform(a.runner)
	var groupIDs []int
	if a.goos == hostos.Darwin {
		if groupIDs, err = hostos.DarwinGroupIDs(a.runner); err != nil {
			return install.Target{}, err
		}
	}
	return install.Target{GOOS: a.goos, ServiceUser: serviceUser, StateDir: c.StateDir, ConfigPath: c.Path, Running: exe,
		ProgramFiles: a.getenv("ProgramFiles"), Users: users, GroupIDs: groupIDs, ConfigExists: configExists, SignedWithRuntime: a.signedWithRuntime(exe)}, nil
}

// signedWithRuntime asks codesign whether the binary already carries the runtime flag (macOS).
func (a *app) signedWithRuntime(binary string) bool {
	if a.goos != hostos.Darwin {
		return false
	}
	info, err := codesignInfo(context.Background(), a.runner, binary)
	return err == nil && info.Runtime
}

// doctorEnv assembles the environment of the service-side checks.
func (a *app) doctorEnv(c *config.Config, st *state.Dir) doctor.Env {
	users, sec, probe := a.platform(a.runner)
	tg := install.Target{GOOS: a.goos, ProgramFiles: a.getenv("ProgramFiles")}
	env := doctor.Env{
		GOOS: a.goos, Config: c, ServiceUser: serviceUser, StateDir: st.Path(), KeyFile: st.File(state.KeysFile),
		Users: users, Security: sec, Resolver: a.resolver, Files: a.files, Probe: probe,
		Helpers:            startup.ChannelHelpers{GOOS: a.goos, Users: users, Services: a.services},
		HelperBinary:       tg.Binary(),
		ServiceDefinitions: tg.ServiceDefinitions(c.SessionUsers),
		SystemSearchDirs:   []string{"/usr/bin", "/bin"},
		DockerSocket:       "/var/run/docker.sock",
		PeerCheckProven:    buildinfo.PeerCheckProven(a.goos),
		Codesign: func(ctx context.Context, b string) (doctor.CodesignInfo, error) {
			return codesignInfo(ctx, a.runner, b)
		},
		InterfaceAddrs: interfaceAddrs,
		LookupIP: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
	}
	if dir := localchan.EndpointDir(a.goos); dir != "" {
		env.EndpointDirs = []string{dir}
	}
	if a.goos == hostos.Windows {
		env.SystemSearchDirs = []string{a.getenv("SystemRoot") + `\System32`}
		env.SystemDrive = a.getenv("SystemDrive")
		env.ServiceUser = install.HostAccountWindows
	}
	return env
}
