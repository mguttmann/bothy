// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The exit codes of 22 10.3.
const (
	ExitOK      = 0
	ExitAborted = 1
	ExitUsage   = 2
	ExitLease   = 3
)

// tokenVariable is the environment variable of the bot token (22 4.2); it is not a configuration
// key.
const tokenVariable = "BOTHY_IMPORT_DISCORD_TOKEN" // #nosec G101 -- the name of the variable, not a token

// Sleeper waits d or until ctx ends and returns the context error in the second case.
type Sleeper func(ctx context.Context, d time.Duration) error

// Env are the process inputs and the dependencies of a command; tests replace them.
type Env struct {
	// Environ is the process environment as os.Environ returns it.
	Environ []string
	// Stdout receives the report; Stderr the log lines and errors.
	Stdout, Stderr io.Writer
	// Version is the version of the tool for the User-Agent.
	Version string
	// SessionPath returns the default session file (bothyapi.DefaultSessionPath).
	SessionPath func() (string, error)
	// Clock is the time of the run lease and of the rate-limit guards of Discord and the CDN; nil
	// means clock.System(). The server client takes its clock from Server.Clock.
	Clock clock.Clock
	// Sleep waits for rate limits of Discord, the CDN and the server; nil means a timer.
	Sleep Sleeper
	// LeaseSleep waits between two lease renewals; nil means Sleep.
	LeaseSleep Sleeper
	// Logger receives the debug log of the clients; nil discards it.
	Logger *slog.Logger
	// Server are the dependencies of the server client (resolver, roots, the clock of its DPoP
	// proofs); its logger and sleeper come from this Env.
	Server bothyapi.Deps
	// Discord are the network dependencies of the Discord client.
	Discord discord.Deps
	// CDN are the network dependencies of the CDN fetcher (resolver, roots); its clock, logger
	// and sleeper come from this Env.
	CDN discord.Deps
	// DiscordEgress and CDNEgress replace the egress values of the clients (tests open the
	// private address of a fake through allowed_hosts); zero means none.
	DiscordEgress, CDNEgress egress.Config
	// AfterBatch is a test hook: called after every successful thread batch before its cursor; an
	// error ends the run there (the crash test of 22 16). nil in production.
	AfterBatch func(batch int) error
	// FollowSleep waits between two rounds of the read mirror (22 12.2), in pieces of at most
	// import.follow.poll_interval; nil means Sleep. Cancelling its context (SIGINT, SIGTERM) ends
	// the mirror after the current batch.
	FollowSleep Sleeper
}

// usage is the help of "bothy import discord".
const usage = `Usage: bothy import discord --guild ID --map FILE [--apply] [--follow | --once] [--channel ID]...
                           [--only-thread ID]... [--report text|json] [--takeover] [--discord-token-file PATH]
                           [--config FILE] [--session-file PATH]
       bothy import discord init-map --guild ID --out FILE [--discord-token-file PATH] [--config FILE]
       bothy import discord status --guild ID [--config FILE] [--session-file PATH]

Without --apply the run is a dry run: it only reads Discord and the server, downloads no attachment,
writes no file (except the rotated session file) and prints the check report. With --apply it
imports into the running source of this server that the owner created and started in the web
console. --follow runs the read mirror: a round every import.follow.poll_interval that copies the
settled new Discord messages of the channels whose lead is discord (with --apply; without it the
round only reads and reports what it would copy); --once runs exactly one round. The bot token comes from BOTHY_IMPORT_DISCORD_TOKEN or --discord-token-file (one of them).
--config names a YAML file with nested import.* keys; BOTHY_IMPORT__... variables win.

Exit codes: 0 done; 1 aborted (network, rate limit above the limit, server error, source paused),
a new run resumes; 2 invocation, mapping file or preflight faulty, nothing written; 3 another run
holds the lease of the source (use --takeover; with --once the normal case for scheduled calls).
`

// reportJSON is the --report value of the JSON report.
const reportJSON = "json"

// stringList is a repeatable flag.
type stringList []string

// String joins the values.
func (s *stringList) String() string { return strings.Join(*s, ",") }

// Set appends a value.
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// options are the parsed flags of a run.
type options struct {
	guild, mapFile, report, tokenFile, configFile, sessionFile string
	apply, takeover, follow, once                              bool
	channels, threads                                          stringList
}

// Main runs "bothy import discord" with the arguments after "discord" and returns the exit code.
func Main(ctx context.Context, args []string, env Env) int {
	if len(args) > 0 {
		switch args[0] {
		case "init-map":
			return initMapMain(ctx, args[1:], env)
		case "status":
			return statusMain(ctx, args[1:], env)
		case "-h", "-help", "--help", "help":
			_, _ = io.WriteString(env.Stdout, usage)
			return ExitOK
		}
	}
	opts, code := parseRun(args, env)
	if code >= 0 {
		return code
	}
	return exitCode(env.Stderr, run(ctx, opts, env))
}

// parseRun parses and checks the flags of a run (brief design note 1 step a): contradictions and
// non-digit IDs end with exit 2 before any call.
func parseRun(args []string, env Env) (options, int) {
	var o options
	fs := flag.NewFlagSet("bothy import discord", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.guild, "guild", "", "")
	fs.StringVar(&o.mapFile, "map", "", "")
	fs.BoolVar(&o.apply, "apply", false, "")
	fs.Var(&o.channels, "channel", "")
	fs.Var(&o.threads, "only-thread", "")
	fs.StringVar(&o.report, "report", "text", "")
	fs.BoolVar(&o.takeover, "takeover", false, "")
	fs.BoolVar(&o.follow, "follow", false, "")
	fs.BoolVar(&o.once, "once", false, "")
	fs.StringVar(&o.tokenFile, "discord-token-file", "", "")
	fs.StringVar(&o.configFile, "config", "", "")
	fs.StringVar(&o.sessionFile, "session-file", "", "")
	if code := parseFlags(fs, args, env); code >= 0 {
		return o, code
	}
	var problem string
	switch {
	case o.guild == "" || o.mapFile == "":
		problem = "--guild and --map are required"
	case !snowflake(o.guild):
		problem = "--guild is not a Discord ID (digits)"
	case o.report != "text" && o.report != reportJSON:
		problem = "--report must be text or json"
	case o.takeover && !o.apply:
		problem = "--takeover needs --apply"
	case o.follow && o.once:
		// 22 10.2 rule 3: contradictory options end before any call.
		problem = "--follow and --once contradict each other"
	case o.follow && len(o.threads) > 0:
		problem = "--only-thread and --follow contradict each other"
	}
	for _, id := range append(append([]string{}, o.channels...), o.threads...) {
		if problem == "" && !snowflake(id) {
			problem = "--channel and --only-thread take Discord IDs (digits)"
		}
	}
	if problem != "" {
		_, _ = fmt.Fprintf(env.Stderr, "bothy import discord: %s\n", problem)
		return o, ExitUsage
	}
	return o, -1
}

// parseFlags parses fs; code is not -1 when the caller must return it.
func parseFlags(fs *flag.FlagSet, args []string, env Env) int {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(env.Stdout, usage)
			return ExitOK
		}
		_, _ = fmt.Fprintf(env.Stderr, "bothy import discord: %s\n", printable(err.Error()))
		return ExitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(env.Stderr, "bothy import discord: unexpected argument %q\n", printable(fs.Arg(0)))
		return ExitUsage
	}
	return -1
}

// printable cuts a user-supplied text for an error line and drops control characters.
func printable(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > 80 {
		s = string(r[:80])
	}
	return s
}

// snowflake reports a Discord ID: 1 to 20 digits that fit an unsigned 64-bit integer.
func snowflake(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	var n uint64
	_, err := fmt.Sscan(s, &n)
	return err == nil
}

// exitCode prints err and maps it to the exit codes of 22 10.3.
func exitCode(stderr io.Writer, err error) int {
	if err == nil {
		return ExitOK
	}
	_, _ = fmt.Fprintf(stderr, "bothy import discord: %s\n", err.Error())
	var u *usageError
	var l *leaseError
	switch {
	case errors.As(err, &u), errors.Is(err, errSelection):
		return ExitUsage
	case errors.As(err, &l):
		return ExitLease
	}
	for _, usage := range []error{bothyapi.ErrNotLoggedIn, bothyapi.ErrSessionRevoked, bothyapi.ErrInsecureSessionFile,
		bothyapi.ErrInvalidSessionFile, bothyapi.ErrInvalidServerURL} {
		if errors.Is(err, usage) {
			return ExitUsage
		}
	}
	return ExitAborted
}

// loadInputs loads the configuration and the bot token (brief design note 1 step b).
func loadInputs(configFile, tokenFile string, env Env) (Config, discord.Token, error) {
	cfg, err := LoadConfig(configFile, env.Environ)
	if err != nil {
		return Config{}, discord.Token{}, &usageError{msg: "configuration: " + err.Error()}
	}
	tok, err := readToken(tokenFile, env.Environ)
	return cfg, tok, err
}

// readToken reads the bot token from BOTHY_IMPORT_DISCORD_TOKEN or the token file: both or neither
// is exit 2, and a token file group or others can read is refused like the session file.
func readToken(file string, environ []string) (discord.Token, error) {
	fromEnv, inEnv := "", false
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, tokenVariable+"="); ok {
			fromEnv, inEnv = v, v != ""
		}
	}
	switch {
	case inEnv && file != "":
		return discord.Token{}, finding("give the bot token either in %s or with --discord-token-file, not both", tokenVariable)
	case !inEnv && file == "":
		return discord.Token{}, finding("the bot token is missing: set %s or use --discord-token-file", tokenVariable)
	case inEnv:
		return tokenOf(fromEnv)
	}
	info, err := os.Stat(file)
	if err != nil {
		return discord.Token{}, finding("the token file cannot be read")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return discord.Token{}, finding("the token file is accessible to group or others; use mode 0600")
	}
	b, err := os.ReadFile(file) // #nosec G304 -- the path of --discord-token-file, chosen by the owner
	if err != nil {
		return discord.Token{}, finding("the token file cannot be read")
	}
	return tokenOf(strings.TrimSpace(string(b)))
}

// tokenOf wraps a token; its error never repeats the value.
func tokenOf(s string) (discord.Token, error) {
	t, err := discord.NewToken(s)
	if err != nil {
		return discord.Token{}, finding("the bot token is not valid (empty, or with white space or control characters)")
	}
	return t, nil
}

// clients are the two clients of a run.
type clients struct {
	dc  *discord.Client
	api *bothyapi.Client
}

// newDiscord builds the Discord client of the configuration.
func newDiscord(cfg Config, tok discord.Token, env Env) (*discord.Client, error) {
	eg := env.DiscordEgress
	if eg.Timeout == 0 {
		eg.Timeout = cfg.HTTPTimeout
	}
	if eg.MaxResponseBytes == 0 {
		eg.MaxResponseBytes = bothyapi.DefaultLimits().MaxResponseBytes
	}
	deps := env.Discord
	deps.Clock, deps.Logger = env.clock(), env.logger()
	if env.Sleep != nil {
		deps.Sleep = discord.Sleeper(env.Sleep)
	}
	return discord.New(discord.Config{APIBase: cfg.DiscordAPIBase, UserAgentVersion: env.version(), PageSize: cfg.DiscordPageSize,
		MaxRetries: cfg.DiscordMax429Retries, MaxRateWait: cfg.DiscordMaxRateWait, HTTPTimeout: cfg.HTTPTimeout, Egress: eg}, tok, deps)
}

// newBothy builds the server client of the session file.
func newBothy(ctx context.Context, cfg Config, sessionFile string, env Env) (*bothyapi.Client, error) {
	path := sessionFile
	if path == "" {
		p, err := env.SessionPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	// The client keeps its own clock (env.Server.Clock, nil for the system clock): the server checks
	// the time in its DPoP proofs against its own clock, while env.Clock is the time of the lease.
	deps := env.Server
	deps.Logger = env.logger()
	if env.Sleep != nil {
		deps.Sleep = bothyapi.Sleeper(env.Sleep)
	}
	lim := bothyapi.DefaultLimits()
	lim.LoginPollInterval, lim.Max429Retries, lim.MaxRateWait, lim.HTTPTimeout =
		cfg.LoginPollInterval, cfg.DiscordMax429Retries, cfg.DiscordMaxRateWait, cfg.HTTPTimeout
	return bothyapi.New(ctx, bothyapi.Options{SessionFile: path, Limits: lim, Deps: deps})
}

// clock returns the clock of the env.
func (e Env) clock() clock.Clock {
	if e.Clock == nil {
		return clock.System()
	}
	return e.Clock
}

// logger returns the debug logger of the env.
func (e Env) logger() *slog.Logger {
	if e.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return e.Logger
}

// version returns the version for the User-Agent.
func (e Env) version() string {
	if e.Version == "" {
		return "0.0.0"
	}
	return e.Version
}

// sleeper returns the sleeper of the env.
func (e Env) sleeper() Sleeper {
	if e.Sleep != nil {
		return e.Sleep
	}
	return func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
}

// leaseSleeper returns the sleeper of the lease ticker.
func (e Env) leaseSleeper() Sleeper {
	if e.LeaseSleep != nil {
		return e.LeaseSleep
	}
	return e.sleeper()
}

// run is the flow of brief design note 1: configuration and token, mapping, clients, preflight,
// with --apply the source, the mapping comparison and the lease, then the walk and the report.
func run(ctx context.Context, o options, env Env) error {
	cfg, tok, err := loadInputs(o.configFile, o.tokenFile, env)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(o.mapFile) // #nosec G304 -- the path of --map, chosen by the owner
	if err != nil {
		return finding("the mapping file cannot be read")
	}
	m, err := ParseMapping(data, o.guild)
	if err != nil {
		return &usageError{msg: err.Error()}
	}
	dc, err := newDiscord(cfg, tok, env)
	if err != nil {
		return &usageError{msg: "configuration: " + err.Error()}
	}
	api, err := newBothy(ctx, cfg, o.sessionFile, env)
	if err != nil {
		return err
	}
	return execute(ctx, o, env, cfg, m, clients{dc: dc, api: api})
}

// execute runs the preflight, the plan and the walk with the built clients.
func execute(ctx context.Context, o options, env Env, cfg Config, m *Mapping, cl clients) error {
	gs, _, err := preflightDiscord(ctx, cl.dc, m, cfg.DiscordPreflightSample, cfg.DiscordPageSize)
	if err != nil {
		return err
	}
	reads := bothyReads{api: cl.api}
	s, err := preflightBothy(ctx, reads, m, gs)
	if err != nil {
		return err
	}
	p := buildStructure(gs, m)
	if err := p.readThreads(ctx, cl.dc, gs, cfg.DiscordClosedTags); err != nil {
		if errors.Is(err, discord.ErrForbidden) {
			return finding("preflight: a mapped channel is not readable by the bot (403 on its thread list)")
		}
		return err
	}
	if err := p.selectScope(o.channels, o.threads); err != nil {
		return err
	}
	wk, err := newWalker(o, env, cfg, m, gs, p, s, cl)
	if err != nil {
		return err
	}
	if o.follow || o.once {
		return follow(ctx, o, env, wk, s.source)
	}
	rep := newReport(o.apply, gs, p)
	wk.rep = rep
	if err := walk(ctx, o, env, wk); err != nil {
		return err
	}
	if o.report == reportJSON {
		return rep.writeJSON(env.Stdout)
	}
	return rep.writeText(env.Stdout)
}

// newWalker builds the walker with the limits of the server (brief design notes 2 and 8): the
// effective import.attachment_max_bytes and import.batch_max_items, each lowered to the server
// limit with a note.
func newWalker(o options, env Env, cfg Config, m *Mapping, gs *guildState, p *plan, s *session, cl clients) (*walker, error) {
	wk := &walker{cfg: cfg, m: m, gs: gs, p: p, dc: cl.dc, api: cl.api, log: env.logger(), afterBatch: env.AfterBatch,
		known: map[string]*refInfo{}, targets: map[string]restclient.Uuid{}, skipped: map[string]bool{}, userDone: map[string]bool{},
		maxContent: s.limits.MessageMaxContentBytes, attachMax: s.limits.FileMaxUploadBytes, batchMax: cfg.BatchMaxItems}
	if cfg.AttachmentMaxBytes > 0 {
		wk.attachMax = cfg.AttachmentMaxBytes
		if cfg.AttachmentMaxBytes > s.limits.FileMaxUploadBytes {
			wk.attachMax = s.limits.FileMaxUploadBytes
			p.notes = append(p.notes, noteAttachmentLowered)
		}
	}
	if wk.batchMax > maxBatchItems {
		wk.batchMax = maxBatchItems
		p.notes = append(p.notes, noteBatchLowered)
	}
	if !o.apply {
		return wk, nil
	}
	if err := applySource(m, s); err != nil {
		return nil, err
	}
	wk.src = s.source.Id
	fetch, err := discord.NewFetcher(discord.CDNConfig{Hosts: cfg.DiscordCDNHosts, MaxBytes: wk.attachMax, Timeout: cfg.HTTPTimeout,
		MaxRetries: cfg.DiscordMax429Retries, MaxRateWait: cfg.DiscordMaxRateWait, UserAgentVersion: env.version(),
		Egress: cdnEgress(cfg, env)}, cl.dc, cdnDeps(env))
	if err != nil {
		return nil, &usageError{msg: "configuration: " + err.Error()}
	}
	wk.up = &uploader{api: cl.api, src: wk.src, fetch: fetch, max: wk.attachMax, posted: map[string]restclient.Uuid{}}
	wk.w = &writer{api: cl.api, reads: bothyReads{api: cl.api}, src: wk.src, clk: env.clock(), runID: newRunID(), stale: cfg.LeaseStaleAfter}
	return wk, nil
}

// maxBatchItems is the maxItems of ImportBatch in the contract (docs/api/openapi.yaml); a larger
// import.batch_max_items is lowered to it.
const maxBatchItems = 100

// cdnEgress returns the egress values of the fetcher.
func cdnEgress(cfg Config, env Env) egress.Config {
	eg := env.CDNEgress
	if eg.Timeout == 0 {
		eg.Timeout = cfg.HTTPTimeout
	}
	if eg.MaxResponseBytes == 0 {
		eg.MaxResponseBytes = bothyapi.DefaultLimits().MaxResponseBytes
	}
	return eg
}

// cdnDeps returns the dependencies of the fetcher.
func cdnDeps(env Env) discord.Deps {
	d := env.CDN
	d.Clock, d.Logger = env.clock(), newCDNLogger(env.logger())
	if env.Sleep != nil {
		d.Sleep = discord.Sleeper(env.Sleep)
	}
	return d
}

// applySource holds the rules of brief design note 3 for --apply: the source of this guild exists,
// is running, and its stored mapping equals the file apart from the leads.
func applySource(m *Mapping, s *session) error {
	src := s.source
	switch {
	case src == nil:
		return finding("no import source exists for this Discord server: create and start it in the web console")
	case string(src.State) != "running":
		return finding("the import source is %s: start the source in the web console", src.State)
	}
	if ptr := m.DiffStored(map[string]any(src.Config)); ptr != "" {
		return finding("the mapping file differs from the mapping stored in the source at %s", ptr)
	}
	return nil
}

// walk runs the walk: in a dry run only reads; with --apply the lease, the renewal ticker, the
// structure, the channels and the release of the lease with the final counters.
func walk(ctx context.Context, o options, env Env, wk *walker) error {
	if !o.apply {
		if err := wk.structure(ctx); err != nil {
			return err
		}
		return wk.channels(ctx)
	}
	if err := wk.w.acquire(ctx, o.takeover); err != nil {
		return err
	}
	wk.present = wk.w.server
	if err := wk.loadPostedFiles(ctx); err != nil {
		return err
	}
	runCtx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wk.w.renewLoop(runCtx, wk.cfg.FollowPollInterval, env.leaseSleeper(), stop)
	}()
	err := wk.structure(runCtx)
	if err == nil {
		err = wk.channels(runCtx)
	}
	if cause := context.Cause(runCtx); err != nil && cause != nil && !errors.Is(cause, context.Canceled) {
		err = cause
	}
	stop(nil)
	<-done
	if err != nil {
		return err
	}
	return wk.w.release(ctx, wk.rep.stats())
}

// loadPostedFiles reads the file items an earlier run posted, so a resumed run reuses them without
// fetching again (brief design note 7).
func (wk *walker) loadPostedFiles(ctx context.Context) error {
	items, err := bothyReads{api: wk.api}.items(ctx, wk.src, "posted")
	if err != nil {
		return err
	}
	for k, it := range items {
		if strings.HasPrefix(k, keyFile) && it.TargetId != nil {
			wk.up.posted[k] = *it.TargetId
		}
	}
	return nil
}
