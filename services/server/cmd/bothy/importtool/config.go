// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// configChapter is the chapter whose table "Configuration values of this chapter" owns every key
// of the tool (22 19).
const configChapter = "22-import-migration.md"

// configEnvPrefix starts every variable of a tool key. Only these variables reach config.Load: a
// server variable on the same machine (BOTHY_DATABASE__URL, BOTHY_CONFIG_FILE) would otherwise
// abort the tool as unknown or name the server file, and BOTHY_IMPORT_DISCORD_TOKEN (one
// underscore) is the token, not a key (D-W-51).
const configEnvPrefix = "BOTHY_IMPORT__"

// Names of the keys the cross-checks and the Config fields read.
const (
	keyAPIBase            = "import.discord.api_base"
	keyCDNHosts           = "import.discord.cdn_hosts"
	keyPageSize           = "import.discord.page_size"
	keyMax429Retries      = "import.discord.max_429_retries"
	keyMaxRateWait        = "import.discord.max_rate_wait"
	keyPreflightSample    = "import.discord.preflight_sample"
	keyClosedTags         = "import.discord.closed_tags"
	keyAttachmentMaxBytes = "import.attachment_max_bytes"
	keyHTTPTimeout        = "import.http_timeout"
	keyLoginPollInterval  = "import.login_poll_interval"
	keyBatchMaxItems      = "import.batch_max_items"
	keyReplyQuoteMaxChars = "import.reply_quote_max_chars"
	keyFollowPoll         = "import.follow.poll_interval"
	keyFollowSettle       = "import.follow.settle_after"
	keyFollowBusyReacts   = "import.follow.busy_reactions"
	keyFollowBusyLookback = "import.follow.busy_lookback"
	keyFollowMaxBackoff   = "import.follow.max_backoff"
	keyLeaseStaleAfter    = "import.lease_stale_after"
)

// Config are the effective tool values (22 19).
type Config struct {
	// DiscordAPIBase is import.discord.api_base.
	DiscordAPIBase string
	// DiscordCDNHosts is import.discord.cdn_hosts.
	DiscordCDNHosts []string
	// DiscordPageSize is import.discord.page_size.
	DiscordPageSize int
	// DiscordMax429Retries is import.discord.max_429_retries.
	DiscordMax429Retries int
	// DiscordMaxRateWait is import.discord.max_rate_wait.
	DiscordMaxRateWait time.Duration
	// DiscordPreflightSample is import.discord.preflight_sample.
	DiscordPreflightSample int
	// DiscordClosedTags is import.discord.closed_tags.
	DiscordClosedTags []string
	// AttachmentMaxBytes is import.attachment_max_bytes; 0 means the server's
	// file.max_upload_bytes from getInstance.
	AttachmentMaxBytes int64
	// HTTPTimeout is import.http_timeout.
	HTTPTimeout time.Duration
	// LoginPollInterval is import.login_poll_interval.
	LoginPollInterval time.Duration
	// BatchMaxItems is import.batch_max_items.
	BatchMaxItems int
	// ReplyQuoteMaxChars is import.reply_quote_max_chars.
	ReplyQuoteMaxChars int
	// FollowPollInterval is import.follow.poll_interval.
	FollowPollInterval time.Duration
	// FollowSettleAfter is import.follow.settle_after.
	FollowSettleAfter time.Duration
	// FollowBusyReactions is import.follow.busy_reactions.
	FollowBusyReactions []string
	// FollowBusyLookback is import.follow.busy_lookback.
	FollowBusyLookback int
	// FollowMaxBackoff is import.follow.max_backoff.
	FollowMaxBackoff time.Duration
	// LeaseStaleAfter is import.lease_stale_after.
	LeaseStaleAfter time.Duration
}

// configKeys are the keys of 22 19 with the defaults of its table. Every duration is at least
// one millisecond and every count at least one (retries may be zero), so a value can never stop
// a loop or turn a wait into a busy spin.
func configKeys() []config.Key {
	key := func(name string, kind config.Kind, def, minimum, maximum, doc string) config.Key {
		return config.Key{Name: name, Kind: kind, Default: def, Min: minimum, Max: maximum, Doc: doc, Chapter: configChapter}
	}
	const ms = "1ms"
	return []config.Key{
		key(keyAPIBase, config.KindURL, "https://discord.com/api/v10", "", "", "Base URL of the Discord API the tool reads from."),
		key(keyCDNHosts, config.KindStringList, "cdn.discordapp.com,media.discordapp.net", "", "", "Hosts the tool may fetch attachments from."),
		key(keyPageSize, config.KindInt, "100", "1", "100", "Messages per page of a Discord message list."),
		key(keyMax429Retries, config.KindInt, "5", "0", "", "Retries of one Discord call after a 429 answer."),
		key(keyMaxRateWait, config.KindDuration, "60s", ms, "", "Longest rate-limit wait before the run aborts."),
		key(keyPreflightSample, config.KindInt, "20", "1", "", "Messages the intent probe of the preflight looks at."),
		key(keyClosedTags, config.KindStringList, "", "", "", "Forum tags that additionally mark a thread as done."),
		key(keyAttachmentMaxBytes, config.KindString, "", "", "", "Largest imported attachment in bytes; empty means the server's file.max_upload_bytes."),
		key(keyHTTPTimeout, config.KindDuration, "60s", ms, "", "Time limit of each call to Discord, the CDN and Bothy."),
		key(keyLoginPollInterval, config.KindDuration, "5s", ms, "", "Interval between polls of completePairing during bothy import login."),
		key(keyBatchMaxItems, config.KindInt, "100", "1", "", "Items per batch sent to the server."),
		key(keyReplyQuoteMaxChars, config.KindInt, "100", "1", "", "Length of the quote line for replies across threads."),
		key(keyFollowPoll, config.KindDuration, "30s", ms, "", "Interval between mirror rounds and between lease renewals."),
		key(keyFollowSettle, config.KindDuration, "60s", ms, "", "Quiet time after the last change before a mirror round takes a thread over."),
		key(keyFollowBusyReacts, config.KindStringList, "\U0001F440", "", "", "Reactions that mean an agent is still working."),
		key(keyFollowBusyLookback, config.KindInt, "10", "1", "", "Messages before the first new one that are checked for a busy reaction."),
		key(keyFollowMaxBackoff, config.KindDuration, "5m", ms, "", "Longest interval after failed mirror rounds."),
		key(keyLeaseStaleAfter, config.KindDuration, "90s", ms, "", "Age after which a run lease no longer blocks a second writing run."),
	}
}

// ConfigSchema returns the schema of the tool keys with their cross-checks (tests and docs).
// Errors: only a defect of the key table itself, which a test holds.
func ConfigSchema() (*config.Schema, error) {
	s, err := config.NewSchema(configKeys())
	if err != nil {
		return nil, fmt.Errorf("import tool configuration: %w", err)
	}
	if err := s.AddCheck("attachment_max_bytes_positive", []string{keyAttachmentMaxBytes}, checkAttachmentMaxBytes); err != nil {
		return nil, fmt.Errorf("import tool configuration: %w", err)
	}
	if err := s.AddCheck("lease_longer_than_poll", []string{keyLeaseStaleAfter, keyFollowPoll}, checkLeaseLongerThanPoll); err != nil {
		return nil, fmt.Errorf("import tool configuration: %w", err)
	}
	return s, nil
}

// errAttachmentMaxBytes is the problem text of a bad import.attachment_max_bytes; like every
// problem it never repeats the value.
var errAttachmentMaxBytes = errors.New("must be empty or a positive base-10 integer")

// checkAttachmentMaxBytes refuses a non-empty import.attachment_max_bytes that is no positive
// base-10 integer. The key is a string because its default, "the server's limit", is only known
// after getInstance; an int key could not express "unset".
func checkAttachmentMaxBytes(v *config.Values) error {
	if _, err := parseAttachmentMaxBytes(v.String(keyAttachmentMaxBytes)); err != nil {
		return err
	}
	return nil
}

// parseAttachmentMaxBytes returns the byte limit of text, 0 for the empty text. Errors:
// errAttachmentMaxBytes for anything but digits without sign and leading zero that fit int64.
func parseAttachmentMaxBytes(text string) (int64, error) {
	if text == "" {
		return 0, nil
	}
	if strings.TrimLeft(text, "0123456789") != "" || text[0] == '0' {
		return 0, errAttachmentMaxBytes
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, errAttachmentMaxBytes
	}
	return n, nil
}

// checkLeaseLongerThanPoll holds 22 19: the lease is renewed every import.follow.poll_interval,
// so import.lease_stale_after must be larger, or a live run would look stale between renewals.
func checkLeaseLongerThanPoll(v *config.Values) error {
	if v.Duration(keyLeaseStaleAfter) <= v.Duration(keyFollowPoll) {
		return errors.New("import.lease_stale_after must be larger than import.follow.poll_interval")
	}
	return nil
}

// DefaultConfig returns the defaults of 22 19 (LoadConfig("", nil) without error). It panics only
// when the key table is broken, which TestDefaultConfigEqualsLoadWithoutSources holds.
func DefaultConfig() Config {
	c, err := LoadConfig("", nil)
	if err != nil {
		panic("import tool configuration defaults: " + err.Error())
	}
	return c
}

// LoadConfig reads the tool configuration: defaults < the YAML file named by --config (file, ""
// for none) < the environment. Only variables whose name starts with BOTHY_IMPORT__ are passed to
// config.Load (a server variable such as BOTHY_DATABASE__URL or BOTHY_CONFIG_FILE on the same
// machine must not abort the tool or name a file, and BOTHY_IMPORT_DISCORD_TOKEN is the token, not
// a key). The file has nested keys under import:; flat dotted keys, keys outside import.* and
// unknown keys are errors (config.Load rejects them). Errors are returned as is (a
// *config.LoadError lists every problem without values); the caller exits 2.
func LoadConfig(file string, environ []string) (Config, error) {
	s, err := ConfigSchema()
	if err != nil {
		return Config{}, err
	}
	var env []string
	for _, entry := range environ {
		if strings.HasPrefix(entry, configEnvPrefix) {
			env = append(env, entry)
		}
	}
	// The filter keeps BOTHY_CONFIG_FILE out, so an empty File really means "no file".
	v, err := config.Load(s, config.Sources{Env: env, File: file})
	if err != nil {
		return Config{}, err //nolint:wrapcheck // the *config.LoadError is the message the caller prints.
	}
	return configFromValues(v)
}

// boundConfig is Config in the form config.Bind fills: one tagged field per key. Bind checks that
// every int fits into int, and import.attachment_max_bytes stays text until the cross-check has
// accepted it.
type boundConfig struct {
	DiscordAPIBase         string        `config:"import.discord.api_base"`
	DiscordCDNHosts        []string      `config:"import.discord.cdn_hosts"`
	DiscordPageSize        int           `config:"import.discord.page_size"`
	DiscordMax429Retries   int           `config:"import.discord.max_429_retries"`
	DiscordMaxRateWait     time.Duration `config:"import.discord.max_rate_wait"`
	DiscordPreflightSample int           `config:"import.discord.preflight_sample"`
	DiscordClosedTags      []string      `config:"import.discord.closed_tags"`
	AttachmentMaxBytes     string        `config:"import.attachment_max_bytes"`
	HTTPTimeout            time.Duration `config:"import.http_timeout"`
	LoginPollInterval      time.Duration `config:"import.login_poll_interval"`
	BatchMaxItems          int           `config:"import.batch_max_items"`
	ReplyQuoteMaxChars     int           `config:"import.reply_quote_max_chars"`
	FollowPollInterval     time.Duration `config:"import.follow.poll_interval"`
	FollowSettleAfter      time.Duration `config:"import.follow.settle_after"`
	FollowBusyReactions    []string      `config:"import.follow.busy_reactions"`
	FollowBusyLookback     int           `config:"import.follow.busy_lookback"`
	FollowMaxBackoff       time.Duration `config:"import.follow.max_backoff"`
	LeaseStaleAfter        time.Duration `config:"import.lease_stale_after"`
}

// configFromValues copies the loaded values into a Config. Errors: Bind refuses an int that does
// not fit into int (no value in the message).
func configFromValues(v *config.Values) (Config, error) {
	var b boundConfig
	if err := config.Bind(v, &b); err != nil {
		return Config{}, fmt.Errorf("import tool configuration: %w", err)
	}
	maxBytes, err := parseAttachmentMaxBytes(b.AttachmentMaxBytes)
	if err != nil {
		return Config{}, fmt.Errorf("import tool configuration: %s %w", keyAttachmentMaxBytes, err)
	}
	return Config{
		DiscordAPIBase:         b.DiscordAPIBase,
		DiscordCDNHosts:        b.DiscordCDNHosts,
		DiscordPageSize:        b.DiscordPageSize,
		DiscordMax429Retries:   b.DiscordMax429Retries,
		DiscordMaxRateWait:     b.DiscordMaxRateWait,
		DiscordPreflightSample: b.DiscordPreflightSample,
		DiscordClosedTags:      b.DiscordClosedTags,
		AttachmentMaxBytes:     maxBytes,
		HTTPTimeout:            b.HTTPTimeout,
		LoginPollInterval:      b.LoginPollInterval,
		BatchMaxItems:          b.BatchMaxItems,
		ReplyQuoteMaxChars:     b.ReplyQuoteMaxChars,
		FollowPollInterval:     b.FollowPollInterval,
		FollowSettleAfter:      b.FollowSettleAfter,
		FollowBusyReactions:    b.FollowBusyReactions,
		FollowBusyLookback:     b.FollowBusyLookback,
		FollowMaxBackoff:       b.FollowMaxBackoff,
		LeaseStaleAfter:        b.LeaseStaleAfter,
	}, nil
}
