// SPDX-License-Identifier: Apache-2.0

// Package config reads and edits the configuration file of the host service (docs/spec/24-terminal.md
// section 17, docs/DECISIONS.md D-M2-42-3): one YAML file nested like the key names
// (host_service: {session_users: [...], ...}), decoded strictly with go.yaml.in/yaml/v3 like the
// server configuration (M1-07). Unknown keys, duplicate keys and wrong types are errors that name
// the key, never the value. Durations use the grammar of the server: a whole number followed by one
// of ms, s, m, h or d.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"go.yaml.in/yaml/v3"
)

// YAML tags of the nodes this package reads and writes.
const (
	tagStr = "!!str"
	tagSeq = "!!seq"
	tagMap = "!!map"
)

// Profile kinds of HostProfile.kind.
const (
	KindShell   = "shell"
	KindClaude  = "claude"
	KindCommand = "command"
)

// Profile is one item of host_service.profiles. Shell and claude have no program here; a command
// profile names an absolute program (HD7 drops it otherwise) and its arguments.
type Profile struct {
	Name, DisplayName, Kind, Program string
	Args                             []string
}

// Workdir is one working directory of host_service.claude_workdirs: the display name the host
// reports and the path only the helper of its session user resolves.
type Workdir struct {
	DisplayName, Path string
}

// WorkdirList is the list of working directories of one key of host_service.claude_workdirs, in
// file order.
type WorkdirList struct {
	SessionUser string
	Dirs        []Workdir
}

// Config is the decoded configuration file with the defaults of 24 section 17 filled in.
type Config struct {
	// Path is the file the configuration came from.
	Path                    string
	StateDir                string
	SessionUsers            []string
	ExtraForbiddenGroups    []string
	CredentialCheckInterval time.Duration
	EntryScanTimeout        time.Duration
	AuthStatusTimeout       time.Duration
	LocalConfirmTimeout     time.Duration
	Profiles                []Profile
	ClaudeBin               string
	ClaudeWorkdirs          []WorkdirList
	ScrollbackBytes         int
	RequireLocalConfirm     bool
}

// Workdirs returns the configured list of sessionUser, or nil.
func (c *Config) Workdirs(sessionUser string) []Workdir {
	for _, l := range c.ClaudeWorkdirs {
		if l.SessionUser == sessionUser {
			return l.Dirs
		}
	}
	return nil
}

// Platform names the operating system the defaults and paths are for. ProgramData and ProgramFiles
// are the Windows folders (the caller reads them from the environment); they are ignored elsewhere.
type Platform struct {
	GOOS, ProgramData, ProgramFiles string
}

// DefaultPath returns the path of the configuration file of p (D-M2-42-3): Linux
// /etc/bothy-host/config.yaml, macOS /Library/Application Support/bothy-host/config.yaml, Windows
// %ProgramFiles%\bothy-host\config.yaml.
func DefaultPath(p Platform) string {
	switch p.GOOS {
	case "darwin":
		return "/Library/Application Support/bothy-host/config.yaml"
	case "windows":
		return p.ProgramFiles + `\bothy-host\config.yaml`
	}
	return "/etc/bothy-host/config.yaml"
}

// Defaults returns the configuration without a file: the defaults of 24 section 17 for p.
func Defaults(p Platform) Config {
	state := "/var/lib/bothy-host"
	if p.GOOS == "windows" {
		state = p.ProgramData + `\bothy-host`
	}
	return Config{
		StateDir:                state,
		CredentialCheckInterval: 60 * time.Second,
		EntryScanTimeout:        10 * time.Minute,
		AuthStatusTimeout:       30 * time.Second,
		LocalConfirmTimeout:     60 * time.Second,
		Profiles:                []Profile{builtin(KindShell)},
		ScrollbackBytes:         1048576,
	}
}

// KeyError is a configuration error of one key; the text names the key and the rule, never the
// value, because a value can be a secret written into the wrong key.
type KeyError struct {
	Key, Msg string
}

func (e *KeyError) Error() string { return e.Key + ": " + e.Msg }

// ErrInvalid wraps every error of Load except a failure to read the file.
var ErrInvalid = errors.New("invalid configuration")

func invalid(key, msg string) error {
	return fmt.Errorf("%w: %w", ErrInvalid, &KeyError{Key: key, Msg: msg})
}

// Load reads the file at path and returns the configuration with the defaults of p. A missing
// file, a read error or any rule violation is an error; rule violations wrap ErrInvalid and a
// *KeyError naming the key.
func Load(path string, p Platform) (*Config, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the path is the configuration file chosen by the administrator
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	c, err := Parse(b, p)
	if err != nil {
		return nil, err
	}
	c.Path = path
	return c, nil
}

// Parse decodes the YAML document b like Load.
func Parse(b []byte, p Platform) (*Config, error) {
	c := Defaults(p)
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, invalid("host_service", "not a YAML document")
	}
	if len(doc.Content) == 0 {
		return &c, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, invalid("host_service", "the file must be a mapping with the key host_service")
	}
	err := eachKey(root, "", func(key string, v *yaml.Node) error {
		if key != "host_service" {
			return invalid(key, "unknown key")
		}
		if v.Kind != yaml.MappingNode {
			return invalid(key, "must be a mapping")
		}
		return eachKey(v, "host_service.", c.set)
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// eachKey calls fn for every key of the mapping m in order, with prefix prepended; a duplicate
// key or a key that is not a string is an error.
func eachKey(m *yaml.Node, prefix string, fn func(string, *yaml.Node) error) error {
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind != yaml.ScalarNode || k.Tag != tagStr {
			return invalid(prefix+"<key>", "keys must be strings")
		}
		key := prefix + k.Value
		if seen[key] {
			return invalid(key, "duplicate key")
		}
		seen[key] = true
		if err := fn(key, m.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) set(key string, v *yaml.Node) error {
	var err error
	switch key {
	case "host_service.state_dir":
		c.StateDir, err = str(key, v)
	case "host_service.session_users":
		c.SessionUsers, err = strList(key, v)
	case "host_service.extra_forbidden_groups":
		c.ExtraForbiddenGroups, err = strList(key, v)
	case "host_service.credential_check_interval":
		c.CredentialCheckInterval, err = duration(key, v)
	case "host_service.entry_scan_timeout":
		c.EntryScanTimeout, err = duration(key, v)
	case "host_service.auth_status_timeout":
		c.AuthStatusTimeout, err = duration(key, v)
	case "host_service.local_confirm_timeout":
		c.LocalConfirmTimeout, err = duration(key, v)
	case "host_service.profiles":
		c.Profiles, err = profiles(key, v)
	case "host_service.claude_bin":
		c.ClaudeBin, err = str(key, v)
	case "host_service.claude_workdirs":
		c.ClaudeWorkdirs, err = workdirs(key, v)
	case "host_service.scrollback_bytes":
		c.ScrollbackBytes, err = integer(key, v)
	case "host_service.require_local_confirm":
		c.RequireLocalConfirm, err = boolean(key, v)
	default:
		err = invalid(key, "unknown key")
	}
	return err
}

func scalar(key string, v *yaml.Node, tag, what string) (string, error) {
	if v.Kind != yaml.ScalarNode || v.Tag != tag {
		return "", invalid(key, "must be "+what)
	}
	return v.Value, nil
}

func str(key string, v *yaml.Node) (string, error) { return scalar(key, v, tagStr, "a string") }

func strList(key string, v *yaml.Node) ([]string, error) {
	if v.Kind != yaml.SequenceNode {
		return nil, invalid(key, "must be a list of strings")
	}
	out := []string{}
	for _, it := range v.Content {
		s, err := str(key, it)
		if err != nil {
			return nil, invalid(key, "must be a list of strings")
		}
		out = append(out, s)
	}
	return out, nil
}

var durationPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(ms|s|m|h|d)$`)

var durationUnits = map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}

// ParseDuration parses the duration grammar of the server configuration (M1-07): a whole number
// followed by one unit of ms, s, m, h or d. Example: ParseDuration("30s") is 30 seconds.
func ParseDuration(text string) (time.Duration, error) {
	m := durationPattern.FindStringSubmatch(text)
	if m == nil {
		return 0, errors.New("must be a whole number followed by one unit of ms, s, m, h or d")
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	size := durationUnits[m[2]]
	if err != nil || n > int64(1<<63-1)/int64(size) {
		return 0, errors.New("is longer than the largest duration")
	}
	return time.Duration(n) * size, nil
}

func duration(key string, v *yaml.Node) (time.Duration, error) {
	if v.Kind != yaml.ScalarNode {
		return 0, invalid(key, "must be a duration")
	}
	d, err := ParseDuration(v.Value)
	if err != nil {
		return 0, invalid(key, err.Error())
	}
	return d, nil
}

func integer(key string, v *yaml.Node) (int, error) {
	s, err := scalar(key, v, "!!int", "a whole number")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, invalid(key, "must be a whole number of at least 0")
	}
	return n, nil
}

func boolean(key string, v *yaml.Node) (bool, error) {
	s, err := scalar(key, v, "!!bool", "true or false")
	if err != nil {
		return false, err
	}
	return s == "true", nil
}

// profileName is the pattern of HostProfile.name.
var profileName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func builtin(kind string) Profile {
	if kind == KindClaude {
		return Profile{Name: KindClaude, DisplayName: "Claude Code", Kind: KindClaude}
	}
	return Profile{Name: KindShell, DisplayName: "Shell", Kind: KindShell}
}

func profiles(key string, v *yaml.Node) ([]Profile, error) {
	if v.Kind != yaml.SequenceNode {
		return nil, invalid(key, "must be a list")
	}
	out := []Profile{}
	for _, it := range v.Content {
		p, err := profile(key, it)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func profile(key string, it *yaml.Node) (Profile, error) {
	if it.Kind == yaml.ScalarNode {
		if it.Value != KindShell && it.Value != KindClaude {
			return Profile{}, invalid(key, "an item that is a string must be shell or claude")
		}
		return builtin(it.Value), nil
	}
	if it.Kind != yaml.MappingNode {
		return Profile{}, invalid(key, "an item must be shell, claude or a mapping")
	}
	var p Profile
	err := eachKey(it, key+".", func(k string, v *yaml.Node) error {
		var err error
		switch k {
		case key + ".name":
			p.Name, err = str(k, v)
		case key + ".display_name":
			p.DisplayName, err = str(k, v)
		case key + ".kind":
			p.Kind, err = str(k, v)
		case key + ".program":
			p.Program, err = str(k, v)
		case key + ".args":
			p.Args, err = strList(k, v)
		default:
			err = invalid(k, "unknown key")
		}
		return err
	})
	switch {
	case err != nil:
		return Profile{}, err
	case !profileName.MatchString(p.Name) || p.Name == KindShell || p.Name == KindClaude:
		return Profile{}, invalid(key+".name", "must match ^[a-z0-9][a-z0-9_-]*$ and not be shell or claude")
	case p.DisplayName == "":
		return Profile{}, invalid(key+".display_name", "must not be empty")
	case p.Kind != KindCommand:
		return Profile{}, invalid(key+".kind", "must be command")
	case p.Program == "":
		return Profile{}, invalid(key+".program", "must not be empty")
	}
	return p, nil
}

func workdirs(key string, v *yaml.Node) ([]WorkdirList, error) {
	if v.Kind != yaml.MappingNode {
		return nil, invalid(key, "must be a mapping from a session user to a list")
	}
	out := []WorkdirList{}
	err := eachKey(v, key+".", func(k string, list *yaml.Node) error {
		if list.Kind != yaml.SequenceNode {
			return invalid(k, "must be a list of {display_name, path}")
		}
		l := WorkdirList{SessionUser: k[len(key)+1:], Dirs: []Workdir{}}
		for _, it := range list.Content {
			if it.Kind != yaml.MappingNode {
				return invalid(k, "must be a list of {display_name, path}")
			}
			var w Workdir
			err := eachKey(it, k+".", func(kk string, vv *yaml.Node) error {
				var err error
				switch kk {
				case k + ".display_name":
					w.DisplayName, err = str(kk, vv)
				case k + ".path":
					w.Path, err = str(kk, vv)
				default:
					err = invalid(kk, "unknown key")
				}
				return err
			})
			if err != nil {
				return err
			}
			l.Dirs = append(l.Dirs, w)
		}
		out = append(out, l)
		return nil
	})
	return out, err
}

// writeAtomic replaces path with b through a temporary file in the same folder, keeping the mode
// of the old file.
func writeAtomic(path string, b []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return fmt.Errorf("write configuration: %w", err)
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return fmt.Errorf("write configuration: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("write configuration: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	return nil
}
