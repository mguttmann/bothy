// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/buildinfo"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/install"
	"github.com/OWNER/bothy/services/host/internal/pairing"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// cmdPair is "pair --server <url> --code <code> [--ca-file <pem>] [--config <path>]"
// (D-M2-42-16). The code and the keys appear in no output.
func cmdPair(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := newFlags("pair", stderr)
	server := fl.String("server", "", "the server URL (https)")
	code := fl.String("code", "", "the pairing code an owner issued")
	caFile := fl.String("ca-file", "", "a PEM file with additional trusted certificates")
	cfgPath := fl.String("config", "", "path of the configuration file")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 || *server == "" || *code == "" {
		_, _ = fmt.Fprintln(stderr, "usage: bothy-host pair --server <url> --code <code> [--ca-file <pem>]")
		return exitUsage
	}
	c, err := sys.loadConfig(*cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host pair: %v\n", err)
		return exitUsage
	}
	ca, err := readCA(*caFile)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitUsage
	}
	w, conn := sys.identity(ctx, c)
	switch w {
	case wayNone:
		return adminRequired(stderr, "pair")
	case wayService:
		defer func() { _ = conn.Close() }()
		a, err := startup.AskPair(ctx, conn, startup.PairRequest{ServerURL: *server, Code: *code, CAPEM: ca})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "bothy-host pair: the running service did not answer: %v\n", err)
			return exitFailed
		}
		if a.Error != "" {
			_, _ = fmt.Fprintln(stderr, a.Error)
			return exitFailed
		}
		printPaired(stdout, a.Fingerprint, a.HostID)
		return restartAfterPair(ctx, c, stderr)
	case wayChild:
		if sys.goos == hostos.Windows {
			_, _ = fmt.Fprintln(stderr, "bothy-host pair: the host service must run; start it with bothy-host restart")
			return exitUsage
		}
		childArgs := []string{"pair", "--server", *server, "--code", *code, "--config", c.Path}
		if len(ca) > 0 {
			childArgs = append(childArgs, "--ca-file", "-")
		}
		if code := sys.childRun(ctx, childArgs, ca, stdout, stderr); code != exitOK {
			return code
		}
		return restartAfterPair(ctx, c, stderr)
	case wayDirect:
	}
	return directPair(ctx, c, *server, *code, ca, stdout, stderr)
}

// readCA reads --ca-file; "-" reads standard input (the way the parent hands the certificates to
// its child as the service user).
func readCA(path string) ([]byte, error) {
	if path != "-" {
		return pairing.ReadCA(path)
	}
	b, err := io.ReadAll(sys.stdin)
	if err != nil || len(b) == 0 {
		return nil, errors.New("pair: no CA certificates on standard input")
	}
	return b, nil
}

func printPaired(stdout io.Writer, fp, host string) {
	_, _ = fmt.Fprintf(stdout, "fingerprint: %s\nhost: %s\n", fp, host)
}

// restartAfterPair restarts the host service, so M2-43 logs in with the new keys.
func restartAfterPair(ctx context.Context, c *config.Config, stderr io.Writer) int {
	if c.OwnerAccount != "" {
		return restartOwnerAccount(ctx, io.Discard, stderr)
	}
	steps, err := install.Restart(install.Target{GOOS: sys.goos})
	if err != nil {
		return exitFailed
	}
	return applyOrFail(ctx, stderr, "pair", steps)
}

// directPair pairs as the owner of the state directory; the helpers are asked over their
// endpoints, and an unreachable helper gives its session user an empty list.
func directPair(ctx context.Context, c *config.Config, server, code string, ca []byte, stdout, stderr io.Writer) int {
	st := state.Open(c.StateDir, sys.protector)
	env := sys.doctorEnv(c, st)
	defer closeHelpers(env)
	env.Declaration, _ = st.ReadDeclaration()
	res := doctor.RunService(ctx, env)
	for _, e := range res.Check(doctor.HD10).Entries {
		if e.SessionUser != "" {
			_, _ = fmt.Fprintln(stderr, "pair: working directories follow with the first report")
			break
		}
	}
	out, err := pairing.Pair(ctx, pairing.Options{ServerURL: server, Code: code, CAPEM: ca, State: st, Random: sys.random,
		Platform: buildinfo.Platform(), OSVersion: sys.osVersion(), HostVersion: buildinfo.Version,
		Lists: pairing.Lists{Profiles: res.Profiles, SessionUsers: res.SessionUsers}})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitFailed
	}
	printPaired(stdout, out.Fingerprint, out.HostID)
	return exitOK
}
