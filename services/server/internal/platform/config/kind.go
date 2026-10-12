// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kind is the type of a configuration key. It selects the parser that turns the text of a
// default, an environment variable, a file scalar or a flag into a value, and the Go type the
// accessors of Values and Bind hand out. The grammars are in docs/operations/configuration.md,
// section "Naming rule and formats".
//
// The zero value is no kind: a Key without Kind fails NewSchema, so a forgotten kind never
// silently means "string" (docs/DECISIONS.md M1-07).
type Kind int

// The kinds of configuration values. The comment names the Go type the value has in Values and
// the field type Bind fills.
const (
	KindString     Kind = iota + 1 // any text; string
	KindEnum                       // one of Key.Enum, exact; string
	KindBool                       // true or false; bool
	KindInt                        // base-10 int64; int64 or int
	KindFloat                      // finite decimal; float64
	KindDuration                   // number plus ms, s, m, h or d; time.Duration
	KindStringList                 // comma list or JSON array; []string
	KindCIDRList                   // list of CIDR prefixes without host bits; []netip.Prefix
	KindListenAddr                 // host:port with an IP literal or empty host; string
	KindCron                       // five cron fields in UTC; string
	KindSemVer                     // SemVer 2.0.0 without leading v; string
	KindSecretRef                  // <scheme>:<reference> with scheme file or env; string
	KindURL                        // http or https URL with host; string
)

var kindNames = map[Kind]string{
	KindString:     "string",
	KindEnum:       "enum",
	KindBool:       "bool",
	KindInt:        "int",
	KindFloat:      "float",
	KindDuration:   "duration",
	KindStringList: "list(string)",
	KindCIDRList:   "list(cidr)",
	KindListenAddr: "listen address",
	KindCron:       "cron",
	KindSemVer:     "semver",
	KindSecretRef:  "secret_ref",
	KindURL:        "url",
}

// String returns the name of the kind as the configuration reference writes it ("duration",
// "list(cidr)", "listen address"), or "Kind(<n>)" for a value that is no kind.
func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

func (k Kind) valid() bool {
	_, ok := kindNames[k]
	return ok
}

func (k Kind) isList() bool { return k == KindStringList || k == KindCIDRList }

// hasBounds reports whether Key.Min and Key.Max apply to the kind.
func (k Kind) hasBounds() bool { return k == KindInt || k == KindFloat || k == KindDuration }

// The parsers below return errors with fixed texts or texts built only from schema data (the
// allowed values of an enum). They never quote the input: a value can be a secret placed in the
// wrong key (02 "Configuration loading" item 4).

// parseValue parses text as a value of kind k and returns the value, its canonical text and an
// error. enum is the list of allowed values of an enum key. The canonical text is unique per
// value and parses back to the same value; Values.Text and bothy config print show it.
func parseValue(k Kind, enum []string, text string) (any, string, error) {
	switch k {
	case KindString:
		return text, text, nil
	case KindEnum:
		if !slices.Contains(enum, text) {
			return nil, "", fmt.Errorf("must be one of %s", strings.Join(enum, ", "))
		}
		return text, text, nil
	case KindBool:
		return parseBool(text)
	case KindInt:
		return parseInt(text)
	case KindFloat:
		return parseFloat(text)
	case KindDuration:
		d, err := parseDuration(text)
		if err != nil {
			return nil, "", err
		}
		return d, formatDuration(d), nil
	case KindStringList, KindCIDRList:
		elems, err := splitList(text)
		if err != nil {
			return nil, "", err
		}
		return parseElements(k, elems)
	case KindListenAddr:
		return parseListenAddr(text)
	case KindCron:
		return parseCron(text)
	case KindSemVer:
		if !semverPattern.MatchString(text) {
			return nil, "", errors.New("must be a semantic version (major.minor.patch after SemVer 2.0.0, without a leading v)")
		}
		return text, text, nil
	case KindSecretRef:
		if err := checkSecretRef(text); err != nil {
			return nil, "", err
		}
		return text, text, nil
	case KindURL:
		if err := checkURL(text); err != nil {
			return nil, "", err
		}
		return text, text, nil
	}
	return nil, "", fmt.Errorf("unknown kind %s", k)
}

func parseBool(text string) (any, string, error) {
	switch text {
	case "true":
		return true, text, nil
	case "false":
		return false, text, nil
	}
	return nil, "", errors.New("must be true or false")
}

var intPattern = regexp.MustCompile(`^-?[0-9]+$`)

func parseInt(text string) (any, string, error) {
	if !intPattern.MatchString(text) {
		return nil, "", errors.New("must be a whole number in decimal digits")
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, "", errors.New("is out of the range of a 64-bit integer")
	}
	return n, strconv.FormatInt(n, 10), nil
}

// floatPattern admits decimal notation only: strconv.ParseFloat alone would also accept hex
// floats, underscores with a prefix, "inf" and "NaN".
var floatPattern = regexp.MustCompile(`^-?([0-9]+(\.[0-9]+)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

func parseFloat(text string) (any, string, error) {
	if !floatPattern.MatchString(text) {
		return nil, "", errors.New("must be a decimal number")
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, "", errors.New("is out of the range of a finite 64-bit float")
	}
	return f, strconv.FormatFloat(f, 'g', -1, 64), nil
}

// durationPattern is the whole duration grammar (docs/DECISIONS.md M1-07): one number without
// sign, fraction or leading zero, one unit. Composites such as 1h30m are not allowed, so every
// value has one spelling per unit and an operator reads it without arithmetic.
var durationPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(ms|s|m|h|d)$`)

// durationUnits lists the units from the largest to the smallest; formatDuration picks the first
// that divides the value.
var durationUnits = []struct {
	suffix string
	size   time.Duration
}{
	{"d", 24 * time.Hour},
	{"h", time.Hour},
	{"m", time.Minute},
	{"s", time.Second},
	{"ms", time.Millisecond},
}

func parseDuration(text string) (time.Duration, error) {
	m := durationPattern.FindStringSubmatch(text)
	if m == nil {
		return 0, errors.New("must be a whole number followed by one unit of ms, s, m, h or d")
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	var size time.Duration
	for _, u := range durationUnits {
		if u.suffix == m[2] {
			size = u.size
		}
	}
	if err != nil || n > int64(math.MaxInt64/size) {
		return 0, errors.New("is longer than the largest duration")
	}
	return time.Duration(n) * size, nil
}

// formatDuration writes d with the largest unit that divides it exactly, so equal durations have
// one canonical text ("120s" and "2m" both print as "2m"). Zero prints as "0s".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	for _, u := range durationUnits {
		if d%u.size == 0 {
			return strconv.FormatInt(int64(d/u.size), 10) + u.suffix
		}
	}
	// Unreachable for parsed values, whose smallest unit is the millisecond.
	return d.String()
}

// splitList splits the environment and default form of a list: a JSON array of strings when the
// text starts with "[", else elements separated by commas with surrounding spaces trimmed. Empty
// text is the empty list. Empty elements are left for parseElements to reject.
func splitList(text string) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}
	if strings.HasPrefix(text, "[") {
		var elems []string
		// The JSON error is not passed on: it quotes the offending character of the input.
		if err := json.Unmarshal([]byte(text), &elems); err != nil {
			return nil, errors.New("must be a JSON array of strings or a comma-separated list")
		}
		return elems, nil
	}
	parts := strings.Split(text, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts, nil
}

// parseElements parses the elements of a list of kind k (KindStringList or KindCIDRList), as
// they come from splitList or from a YAML sequence, and returns the list and its canonical text.
// Errors name the position of an element, never its text.
func parseElements(k Kind, elems []string) (any, string, error) {
	texts := make([]string, 0, len(elems))
	prefixes := make([]netip.Prefix, 0, len(elems))
	for i, e := range elems {
		if e == "" {
			return nil, "", fmt.Errorf("element %d is empty", i+1)
		}
		if k == KindCIDRList {
			p, err := netip.ParsePrefix(e)
			if err != nil {
				return nil, "", fmt.Errorf("element %d is not a CIDR prefix", i+1)
			}
			if p != p.Masked() {
				return nil, "", fmt.Errorf("element %d has host bits set", i+1)
			}
			prefixes = append(prefixes, p)
			e = p.String()
		}
		texts = append(texts, e)
	}
	canon := listText(texts)
	if k == KindCIDRList {
		return prefixes, canon, nil
	}
	return texts, canon, nil
}

// listText renders a list in the comma form when every element survives the round trip through
// splitList, else as a JSON array. The empty list is the empty text.
func listText(elems []string) string {
	commaSafe := len(elems) == 0 || !strings.HasPrefix(elems[0], "[")
	for _, e := range elems {
		if strings.Contains(e, ",") || strings.TrimSpace(e) != e {
			commaSafe = false
		}
	}
	if commaSafe {
		return strings.Join(elems, ", ")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// Encoding a []string cannot fail.
	_ = enc.Encode(elems)
	return strings.TrimSuffix(buf.String(), "\n")
}

var portPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,4})$`)

// parsePort accepts a decimal port without leading zeros from low to 65535.
func parsePort(text string, low int) (int, bool) {
	if !portPattern.MatchString(text) {
		return 0, false
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < low || n > math.MaxUint16 {
		return 0, false
	}
	return n, true
}

func parseListenAddr(text string) (any, string, error) {
	host, port, err := net.SplitHostPort(text)
	if err != nil {
		return nil, "", errors.New("must be host:port with an IP address or an empty host")
	}
	p, ok := parsePort(port, 0)
	if !ok {
		return nil, "", errors.New("port must be a decimal number from 0 to 65535")
	}
	if host == "" {
		canon := ":" + strconv.Itoa(p)
		return canon, canon, nil
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return nil, "", errors.New("host must be empty or an IP address")
	}
	canon := net.JoinHostPort(addr.String(), strconv.Itoa(p))
	return canon, canon, nil
}

// cronFields names the five fields with their ranges (UTC, no names for months or weekdays).
var cronFields = []struct {
	name     string
	low, max int
}{
	{"minute", 0, 59},
	{"hour", 0, 23},
	{"day of month", 1, 31},
	{"month", 1, 12},
	{"day of week", 0, 6},
}

var cronNumber = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

func parseCron(text string) (any, string, error) {
	fields := strings.Fields(text)
	if len(fields) != len(cronFields) {
		return nil, "", errors.New("must have five fields: minute, hour, day of month, month, day of week")
	}
	for i, f := range fields {
		spec := cronFields[i]
		if !cronFieldValid(f, spec.low, spec.max) {
			return nil, "", fmt.Errorf("field %s must use *, */n, a, a-b, a-b/n or a comma list of these with values from %d to %d", spec.name, spec.low, spec.max)
		}
	}
	canon := strings.Join(fields, " ")
	return canon, canon, nil
}

// cronFieldValid checks one field: a comma list of "*", "*/n", "a", "a-b" and "a-b/n", values
// from low to high, a <= b, step n from 1 to high.
func cronFieldValid(field string, low, high int) bool {
	for _, part := range strings.Split(field, ",") {
		if !cronPartValid(part, low, high) {
			return false
		}
	}
	return true
}

// cronPartValid checks one element of a cron field: "*", "*/n", "a", "a-b" or "a-b/n".
func cronPartValid(part string, low, high int) bool {
	base, step, hasStep := strings.Cut(part, "/")
	if hasStep {
		if _, ok := cronValue(step, 1, high); !ok {
			return false
		}
	}
	if base == "*" {
		return true
	}
	from, to, isRange := strings.Cut(base, "-")
	if hasStep && !isRange {
		return false
	}
	a, ok := cronValue(from, low, high)
	if !ok {
		return false
	}
	if !isRange {
		return true
	}
	b, ok := cronValue(to, low, high)
	return ok && a <= b
}

// cronValue parses a number of a cron field, without leading zeros, from lo to hi.
func cronValue(s string, lo, hi int) (int, bool) {
	if !cronNumber.MatchString(s) {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= lo && n <= hi
}

// semverPattern is the regular expression published with SemVer 2.0.0 (semver.org, FAQ "Is there
// a suggested regular expression"), anchored, without a leading v.
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

var envVariablePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// errSchemeUnsupported is the text for schemes that 16 "Secret provider" plans for a later
// milestone; the reference is well formed, this server version just has no driver for it.
const errSchemeUnsupported = "scheme not supported by this server version"

// checkSecretRef checks the grammar of a secret reference. A file: path must be absolute and
// clean and must not contain NUL, the same grammar secrets.ParseRef applies (ruling M1-09/5): a
// reference that bothy config check accepts must not fail later in the provider, and an unclean
// path ("/a/../b") would name another file than an operator reading it expects.
func checkSecretRef(text string) error {
	scheme, rest, ok := strings.Cut(text, ":")
	if !ok || scheme == "" {
		return errors.New("must have the form <scheme>:<reference>")
	}
	switch scheme {
	case "file":
		switch {
		case !path.IsAbs(rest):
			return errors.New("file reference must be an absolute path")
		case strings.ContainsRune(rest, 0):
			return errors.New("file reference must not contain a NUL character")
		case path.Clean(rest) != rest:
			return errors.New("file reference must be a clean path without . or .. elements and without repeated or trailing slashes")
		}
	case "env":
		if !envVariablePattern.MatchString(rest) {
			return errors.New("env reference must name a variable of upper-case letters, digits and underscores")
		}
		if !secretVariable(rest) {
			return errors.New("env reference must not name BOTHY_CONFIG_FILE or a variable in the form of a configuration key")
		}
	case "azkv", "awssm":
		return errors.New(errSchemeUnsupported)
	default:
		return errors.New("unknown scheme; this server version supports file and env")
	}
	return nil
}

// secretVariable reports whether an env: reference may name the variable: not BOTHY_CONFIG_FILE
// and not a name that KeyFromEnv maps to a key. Such a variable would be configuration and
// secret at once (bothy config print shows configuration values), and a reference to a mistyped
// key variable would exempt exactly the typo the unknown-variable rule exists for.
func secretVariable(name string) bool {
	_, keyShaped := KeyFromEnv(name)
	return name != configFileVariable && !keyShaped
}

// envReference returns the variable a valid env: reference names.
func envReference(ref string) (string, bool) {
	name, ok := strings.CutPrefix(ref, "env:")
	if !ok || !envVariablePattern.MatchString(name) || !secretVariable(name) {
		return "", false
	}
	return name, true
}

// checkSecretsDir checks secrets.dir: an absolute path in clean form (path.Clean leaves it as it
// is), not the root directory (file:<secrets.dir>/x would become file://x), without NUL and
// without "<", so an expanded reference never holds a placeholder again. The texts never quote
// the value.
func checkSecretsDir(dir string) error {
	switch {
	case !path.IsAbs(dir):
		return errors.New("must be an absolute path")
	case strings.ContainsRune(dir, 0):
		return errors.New("must not contain a NUL character")
	case dir == "/":
		return errors.New("must not be the root directory")
	case strings.Contains(dir, "<"):
		return errors.New("must not contain <")
	case path.Clean(dir) != dir:
		return errors.New("must be a clean path without . or .. elements and without repeated or trailing slashes")
	}
	return nil
}

func checkURL(text string) error {
	u, err := url.Parse(text)
	switch {
	case err != nil:
		return errors.New("must be a URL")
	case u.Scheme != "http" && u.Scheme != "https":
		return errors.New("scheme must be http or https")
	case u.Opaque != "" || u.Hostname() == "":
		return errors.New("must name a host")
	case u.User != nil:
		return errors.New("must not contain user information")
	case strings.Contains(text, "#"):
		return errors.New("must not contain a fragment")
	}
	if p := u.Port(); p != "" {
		if _, ok := parsePort(p, 1); !ok {
			return errors.New("port must be a decimal number from 1 to 65535")
		}
	}
	return nil
}
