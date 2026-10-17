// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// cmdDoctor is "doctor [--config <path>]": HD1 to HD11 with the entry scan; exit 1 as soon as one
// check fails, warnings alone give 0 (D-M2-42-2).
func cmdDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fl := newFlags("doctor", stderr)
	cfgPath := fl.String("config", "", "path of the configuration file")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 {
		return exitUsage
	}
	c, err := sys.loadConfig(*cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy-host doctor: %v\n", err)
		return exitUsage
	}
	w, conn := sys.identity(ctx, c)
	switch w {
	case wayNone:
		return adminRequired(stderr, "doctor")
	case wayService:
		defer func() { _ = conn.Close() }()
		a, err := startup.AskDoctor(ctx, conn)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "bothy-host doctor: the running service did not answer: %v\n", err)
			return exitFailed
		}
		_, _ = fmt.Fprint(stdout, a.Output)
		return a.Exit
	case wayChild:
		return sys.childRun(ctx, []string{"doctor", "--config", c.Path}, nil, stdout, stderr)
	case wayDirect:
	}
	return directDoctor(ctx, c, stdout)
}

// directDoctor runs the checks as the owner of the state directory.
func directDoctor(ctx context.Context, c *config.Config, stdout io.Writer) int {
	st := state.Open(c.StateDir, sys.protector)
	env := sys.doctorEnv(c, st)
	env.Declaration, _ = st.ReadDeclaration()
	env.Bindings, _ = st.ReadBindings()
	if p, err := st.ReadPairing(); err == nil {
		env.ServerURL = p.ServerURL
	}
	r := doctor.NewRunner(env, nil)
	r.RunFull(ctx)
	r.WaitScans()
	res := r.Last()
	_, _ = fmt.Fprint(stdout, res.Format())
	return res.ExitCode()
}
