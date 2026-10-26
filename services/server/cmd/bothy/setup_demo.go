// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/demo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The defaults and the exit code of bothy setup demo (02 3.3, 16 16.1).
const (
	// defaultDemoData is where the demo overlay mounts infra/demo into the api container.
	defaultDemoData = "/usr/share/bothy/demo"
	defaultDemoSeed = 1
	// exitDemoNotEmpty is the exit code for an instance that is not empty (nothing written).
	exitDemoNotEmpty = 3
)

// demoOptions are the flags of bothy setup demo.
type demoOptions struct {
	file, data, api, exchange, pace string
	seed                            uint64
}

// parseDemoFlags parses the flags of setup demo.
func parseDemoFlags(args []string) (demoOptions, error) {
	fs := flag.NewFlagSet("bothy setup demo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o demoOptions
	fs.StringVar(&o.file, "config", "", "configuration file")
	fs.StringVar(&o.data, "data", defaultDemoData, "data set directory")
	fs.StringVar(&o.api, "api", "", "address of the public API")
	fs.StringVar(&o.exchange, "exchange", "", "exchange directory")
	fs.StringVar(&o.pace, "pace", "", "wait between two warm-up prompts")
	fs.Uint64Var(&o.seed, "seed", defaultDemoSeed, "seed")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return demoOptions{}, err
		}
		return demoOptions{}, errors.New(printable(err.Error()))
	}
	if fs.NArg() > 0 {
		return demoOptions{}, fmt.Errorf("unexpected argument %q", printable(fs.Arg(0)))
	}
	if o.file == "" && flagGiven(fs, "config") {
		return demoOptions{}, errors.New("--config needs a file path")
	}
	if o.data == "" {
		return demoOptions{}, errors.New("--data needs a directory")
	}
	return o, nil
}

// demoCommand runs "bothy setup demo [flags]" (16 16.1): it loads and validates the data set
// before it opens any connection (exit 2), opens the setup service like bootstrap-owner, and runs
// demo.Run with the bootstrap of the setup service; an instance whose setup is complete is exit 3
// before any write.
func demoCommand(ctx context.Context, args, env []string, stdout, stderr io.Writer, d setupDeps) int {
	o, err := parseDemoFlags(args)
	if errors.Is(err, flag.ErrHelp) {
		_, _ = io.WriteString(stdout, setupUsage)
		return exitOK
	}
	if err != nil {
		return setupUsageError(stderr, err.Error())
	}
	pace := time.Duration(-1)
	if o.pace != "" {
		if pace, err = time.ParseDuration(o.pace); err != nil || pace < 0 {
			return setupUsageError(stderr, "--pace needs a duration that is not negative")
		}
	}
	ds, err := demo.Load(o.data)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy setup: %v\n", err)
		return exitUsage
	}
	s, values, ok := loadConfig(env, wire.ConfigSchema, configOptions{file: o.file}, stderr)
	if !ok {
		return exitFailure
	}
	log := commandLogger(stderr, s, values)
	api := o.api
	if api == "" {
		if api, err = loopbackAPI(values); err != nil {
			return setupUsageError(stderr, err.Error())
		}
	}
	runner, err := d.open(ctx, values, env, log)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bothy setup: %v\n", err)
		return exitFailure
	}
	defer func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), setupCloseBudget)
		defer cancel()
		runner.Close(cctx)
	}()
	_, err = demo.Run(ctx, demo.Options{
		DataSet: ds, Seed: o.seed, APIURL: api, PublicURL: values.String("server.public_url"), Exchange: o.exchange, Pace: pace,
		FingerprintChars: int(values.Int("terminal.fingerprint_confirm_chars")),
		Bootstrap: func(ctx context.Context, owner demo.Person) (string, error) {
			res, err := runner.BootstrapOwner(ctx, wire.BootstrapOwnerCmd{
				Handle: owner.Handle, DisplayName: owner.DisplayName, Locale: owner.Locale, TimeZone: owner.TimeZone,
			})
			if errors.Is(err, wire.ErrSetupCompleted) {
				return "", demo.ErrNotEmpty
			}
			if err != nil {
				return "", err
			}
			return res.Code.Code, nil
		},
		Stdout: stdout, Log: log,
	})
	switch {
	case errors.Is(err, demo.ErrNotEmpty):
		_, _ = fmt.Fprintln(stderr, "bothy setup: the instance is not empty; nothing written")
		return exitDemoNotEmpty
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "bothy setup: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintln(stderr, "bothy setup: the demo is ready; sign in with the code on stdout, it is valid once")
	return exitOK
}

// loopbackAPI returns http://127.0.0.1 with the port of server.listen_addr: the command runs next
// to the server (16 16.1), which accepts plaintext only from loopback (SEC-TLS-1).
func loopbackAPI(v *config.Values) (string, error) {
	_, port, err := net.SplitHostPort(v.String("server.listen_addr"))
	if err != nil {
		return "", errors.New("server.listen_addr names no port; pass --api")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", errors.New("server.listen_addr names no port; pass --api")
	}
	return "http://" + net.JoinHostPort("127.0.0.1", port), nil
}
