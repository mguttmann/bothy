// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The codes of errors[].code this package uses: the vocabulary of M1-18, and the reasons 06 and
// 09 already name for a host and a reserved command name (M2-24 design note 3 e).
const (
	CodeRequired            = "required"
	CodeInvalid             = "invalid"
	CodeTooLarge            = "too_large"
	CodeTooLong             = "too_long"
	CodeTooManyItems        = "too_many_items"
	CodeHostNotAllowed      = "webhook_host_not_allowed"
	CodeCommandNameReserved = "command_name_reserved"
)

// ScopeEventsSubscribe is the scope every subscribing app needs (05 section 2.6 row
// "Ereignis-Abos").
const ScopeEventsSubscribe = "events:subscribe"

// ReservedCommands are the built-in command names of 09 section 10.1; no manifest may register
// them.
var ReservedCommands = []string{"stop", "status", "new", "resume", "title", "close"}

// Env are the facts ValidateManifest needs beyond the manifest: catalogue facts and
// configuration, gathered by the application layer (M2-24 design note 3 b).
type Env struct {
	// MaxBytes is app.manifest_max_bytes.
	MaxBytes int
	// HandleMaxChars is principal.handle_max_chars: the slug becomes the handle of the app.
	HandleMaxChars int
	// DisplayNameMaxChars is principal.display_name_max_chars: the name becomes its display name.
	DisplayNameMaxChars int
	// MaxCommands is command.max_per_principal.
	MaxCommands int
	// CommandDescriptionMaxChars is command.description_max_chars.
	CommandDescriptionMaxChars int
	// SpecVersion is the version of the API contract the server implements (routes.SpecVersion).
	SpecVersion string
	// AllowedHosts are the parsed entries of webhook.allowed_hosts (ParseAllowedHosts).
	AllowedHosts []AllowedHost
	// KnownScope reports a scope of the catalogue (05 section 2.6).
	KnownScope func(scope string) bool
	// EventScope reports whether an app may subscribe to the event type t and the scope its
	// receive action needs ("" for none); authz.AppScopes.EventScope.
	EventScope func(t string) (scope string, subscribable bool)
}

// The rule names of the details; a detail names the rule, never the submitted value.
const (
	ruleFormat   = "11 section 3.1"
	ruleScopes   = "R-EX-03, 11 section 3.2 rule 1"
	ruleEvents   = "11 section 3.2 rules 1 and 2"
	ruleHost     = "R-DM-17, 11 section 3.2 rule 3"
	ruleGrants   = "R-EX-03, 11 section 3.2 rule 4"
	ruleCommands = "11 section 3.2 rule 5, 09 section 10"
	ruleText     = "SEC-IN-4"
)

// minAPIPattern is min_api_version: major.minor (06 section 13).
var minAPIPattern = regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`)

// checker collects the errors of one manifest.
type checker struct {
	env  Env
	errs []errs.FieldError
}

// add appends one error at pointer.
func (c *checker) add(pointer, code, detail string) {
	c.errs = append(c.errs, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// ValidateManifest applies the check rules of 11 sections 3.1 and 3.2 (M2-24 design notes 4 and
// 5) to m, whose bytes as received are raw, and returns every failure with a JSON Pointer relative
// to the manifest root. It is pure: env carries every catalogue fact and configuration value.
// Raw bytes above env.MaxBytes give one too_large error on the root pointer and stop the check.
// An empty result means the manifest is valid.
//
// Example:
//
//	if fe := domain.ValidateManifest(m, raw, env); len(fe) > 0 {
//		return errs.New(errs.CodeValidationFailed, "").WithFields(fe...)
//	}
func ValidateManifest(m Manifest, raw []byte, env Env) []errs.FieldError {
	c := &checker{env: env}
	if len(raw) > env.MaxBytes {
		c.add("", CodeTooLarge, "The manifest exceeds app.manifest_max_bytes ("+ruleFormat+").")
		return c.errs
	}
	if !storableJSON(raw) {
		c.add("", CodeInvalid, "The manifest holds the escape of NUL or of an unpaired surrogate (SEC-IN-4).")
	}
	c.identity(m)
	c.scopes(m)
	c.events(m)
	c.delivery(m)
	c.grants(m)
	c.commands(m)
	c.lists(m)
	return c.errs
}

// identity checks schema_version, slug, name, the display texts, version and min_api_version.
func (c *checker) identity(m Manifest) {
	if m.SchemaVersion != SchemaVersion {
		c.add("/schema_version", CodeInvalid, "Only schema_version 1 exists ("+ruleFormat+").")
	}
	if utf8.RuneCountInString(m.Slug) > c.env.HandleMaxChars {
		c.add("/slug", CodeTooLong, "The slug exceeds principal.handle_max_chars ("+ruleFormat+").")
	}
	c.displayName("/name", m.Name)
	c.text("/description", m.Description, true)
	c.text("/publisher", m.Publisher, true)
	c.text("/data_egress", m.DataEgress, false)
	if _, ok := ParseSemVer(m.Version); !ok {
		c.add("/version", CodeInvalid, "The version is no SemVer 2.0.0 version ("+ruleFormat+").")
	}
	if !c.apiVersionSupported(m.MinAPIVersion) {
		c.add("/min_api_version", CodeInvalid, "The server implements an older API version ("+ruleFormat+").")
	}
	if m.HomepageURL != "" {
		if _, ok := parseHTTPS(m.HomepageURL); !ok {
			c.add("/homepage_url", CodeInvalid, "The homepage is no absolute https address ("+ruleFormat+").")
		}
	}
}

// apiVersionSupported reports a min_api_version whose major and minor are not above those of the
// server's spec version.
func (c *checker) apiVersionSupported(v string) bool {
	parts := minAPIPattern.FindStringSubmatch(v)
	spec, ok := ParseSemVer(c.env.SpecVersion)
	if parts == nil || !ok {
		return false
	}
	major, err1 := strconv.ParseUint(parts[1], 10, 64)
	minor, err2 := strconv.ParseUint(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return false
	}
	return major < spec.Major || (major == spec.Major && minor <= spec.Minor)
}

// displayName checks the name the app principal shows: the rules of a display name of M1-26.
func (c *checker) displayName(pointer, s string) {
	if err := textcheck.DisplayText(s); err != nil {
		c.add(pointer, CodeInvalid, "The name contains a forbidden character ("+ruleText+").")
		return
	}
	if strings.TrimFunc(s, invisible) == "" {
		c.add(pointer, CodeRequired, "The name must not be blank ("+ruleFormat+").")
		return
	}
	if utf8.RuneCountInString(s) > c.env.DisplayNameMaxChars {
		c.add(pointer, CodeTooLong, "The name exceeds principal.display_name_max_chars ("+ruleFormat+").")
	}
}

// invisible reports runes that show nothing on their own (white space and format characters).
func invisible(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) }

// text checks a display text through kernel/textcheck; required texts must not be blank.
func (c *checker) text(pointer, s string, required bool) {
	if s == "" && !required {
		return
	}
	if err := textcheck.DisplayText(s); err != nil {
		c.add(pointer, CodeInvalid, "The text contains a forbidden character ("+ruleText+").")
		return
	}
	if required && strings.TrimFunc(s, invisible) == "" {
		c.add(pointer, CodeRequired, "The text must not be blank ("+ruleFormat+").")
	}
}

// scopes checks the two scope lists: known scopes, no duplicates, none in both lists.
func (c *checker) scopes(m Manifest) {
	seen := map[string]bool{}
	for _, l := range []struct {
		pointer string
		list    []string
	}{{"/scopes/required", m.Scopes.Required}, {"/scopes/optional", m.Scopes.Optional}} {
		for i, s := range l.list {
			p := l.pointer + "/" + strconv.Itoa(i)
			switch {
			case c.env.KnownScope != nil && !c.env.KnownScope(s):
				c.add(p, CodeInvalid, "The scope is not in the catalogue ("+ruleScopes+").")
			case seen[s]:
				c.add(p, CodeInvalid, "The scope is requested twice or in both lists ("+ruleScopes+").")
			}
			seen[s] = true
		}
	}
}

// events checks the subscribed event types: subscribable, no duplicates, the scope of the receive
// action and events:subscribe requested.
func (c *checker) events(m Manifest) {
	if len(m.Events) == 0 {
		return
	}
	requested := m.AllScopes()
	if !slices.Contains(requested, ScopeEventsSubscribe) {
		c.add("/events", CodeInvalid, "Events need the scope events:subscribe ("+ruleEvents+").")
	}
	seen := map[string]bool{}
	for i, t := range m.Events {
		p := "/events/" + strconv.Itoa(i)
		if seen[t] {
			c.add(p, CodeInvalid, "The event is listed twice ("+ruleEvents+").")
			continue
		}
		seen[t] = true
		scope, ok := "", false
		if c.env.EventScope != nil {
			scope, ok = c.env.EventScope(t)
		}
		switch {
		case !ok:
			c.add(p, CodeInvalid, "Apps cannot subscribe to this event type ("+ruleEvents+").")
		case scope != "" && !slices.Contains(requested, scope):
			c.add(p, CodeInvalid, "The scope of the receive action of this event is not requested ("+ruleEvents+").")
		}
	}
}

// delivery checks the conditional members of the delivery block and its addresses.
func (c *checker) delivery(m Manifest) {
	d := m.Delivery
	if d == nil {
		if len(m.Events) > 0 || len(m.Commands) > 0 || len(m.Components) > 0 {
			c.add("/delivery", CodeRequired, "Events, commands and components need a delivery block ("+ruleFormat+").")
		}
		if len(m.Events) > 0 && m.DataEgress == "" {
			c.add("/data_egress", CodeRequired, "Events need data_egress ("+ruleFormat+").")
		}
		return
	}
	c.deliveryMembers(m, d)
	c.address("/delivery/events_url", d.EventsURL)
	c.address("/delivery/interactions_url", d.InteractionsURL)
}

// deliveryMembers checks which members of the delivery block d the manifest needs (11 3.1 column
// "Pflicht").
func (c *checker) deliveryMembers(m Manifest, d *Delivery) {
	switch {
	case d.Mode == DeliveryWebhook && d.EventsURL == "":
		c.add("/delivery/events_url", CodeRequired, "The webhook mode needs events_url ("+ruleFormat+").")
	case d.Mode != DeliveryWebhook && d.EventsURL != "":
		c.add("/delivery/events_url", CodeInvalid, "Only the webhook mode takes events_url ("+ruleFormat+").")
	}
	if (len(m.Commands) > 0 || len(m.Components) > 0) && d.Mode != DeliveryGateway && d.InteractionsURL == "" {
		c.add("/delivery/interactions_url", CodeRequired, "Commands and components without the gateway need interactions_url ("+ruleFormat+").")
	}
	if (len(m.Events) > 0 || d.InteractionsURL != "") && m.DataEgress == "" {
		c.add("/data_egress", CodeRequired, "Events and interactions need data_egress ("+ruleFormat+").")
	}
}

// address checks a delivery address with ValidateTarget, the rule of subscription addresses and of
// the webhook egress client (docs/DECISIONS.md D-M2-26-8, replacing the host-without-port
// comparison of M2-24 design note 5): a host or port outside webhook.allowed_hosts is
// webhook_host_not_allowed, every other defect invalid.
func (c *checker) address(pointer, raw string) {
	if raw == "" {
		return
	}
	_, err := ValidateTarget(raw, c.env.AllowedHosts)
	switch {
	case errors.Is(err, ErrHostNotAllowed):
		c.add(pointer, CodeHostNotAllowed, "The host or port is not in webhook.allowed_hosts ("+ruleHost+").")
	case err != nil:
		c.add(pointer, CodeInvalid, "The address is no absolute https address without user information and fragment ("+ruleHost+").")
	}
}

// parseHTTPS parses raw as an absolute https URL with a host, without user information and
// without fragment.
func parseHTTPS(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != schemeHTTPS || u.Hostname() == "" || u.User != nil || u.Fragment != "" ||
		strings.Contains(raw, "#") || u.Opaque != "" {
		return nil, false
	}
	return u, true
}

// The grant shapes a manifest may request (R-DM-08, R-DM-20, R-DM-35).
const (
	levelRead      = "read"
	levelWrite     = "write"
	targetHost     = "host"
	capUpload      = "upload"
	targetInstance = "instance"
)

// grants checks requested_grants: no host target, level read or write, capabilities at most
// upload, without duplicates, and a purpose.
func (c *checker) grants(m Manifest) {
	for i, g := range m.RequestedGrants {
		p := "/requested_grants/" + strconv.Itoa(i)
		if g.TargetKind == targetHost {
			c.add(p+"/target_kind", CodeInvalid, "Apps never get a host grant ("+ruleGrants+").")
		}
		if g.Level != levelRead && g.Level != levelWrite {
			c.add(p+"/level", CodeInvalid, "Apps get at most write, never trigger ("+ruleGrants+").")
		}
		seen := map[string]bool{}
		for j, cp := range g.Capabilities {
			cpp := p + "/capabilities/" + strconv.Itoa(j)
			switch {
			case cp != capUpload:
				c.add(cpp, CodeInvalid, "Apps get at most the capability upload ("+ruleGrants+").")
			case seen[cp]:
				c.add(cpp, CodeInvalid, "The capability is listed twice ("+ruleGrants+").")
			}
			seen[cp] = true
		}
		c.text(p+"/purpose", g.Purpose, true)
	}
}

// commands checks the commands: count, unique and unreserved names, descriptions, unique option
// names and no target (11 section 4.1: a manifest never names a target).
func (c *checker) commands(m Manifest) {
	if len(m.Commands) > c.env.MaxCommands {
		c.add("/commands", CodeTooManyItems, "More commands than command.max_per_principal ("+ruleCommands+").")
	}
	seen := map[string]bool{}
	for i, cmd := range m.Commands {
		p := "/commands/" + strconv.Itoa(i)
		name := strings.ToLower(cmd.Name)
		switch {
		case slices.Contains(ReservedCommands, name):
			c.add(p+"/name", CodeCommandNameReserved, "The name is a built-in command ("+ruleCommands+").")
		case seen[name]:
			c.add(p+"/name", CodeInvalid, "The name is used by another command ("+ruleCommands+").")
		}
		seen[name] = true
		c.command(p, cmd)
	}
}

// command checks the description, the target and the options of one command at pointer p.
func (c *checker) command(p string, cmd Command) {
	if utf8.RuneCountInString(cmd.Description) > c.env.CommandDescriptionMaxChars {
		c.add(p+"/description", CodeTooLong, "The description exceeds command.description_max_chars ("+ruleCommands+").")
	} else {
		c.text(p+"/description", cmd.Description, true)
	}
	if cmd.ScopeKind != "" && cmd.ScopeKind != targetInstance {
		c.add(p+"/scope_kind", CodeInvalid, "A manifest never sets the target of a command ("+ruleCommands+").")
	}
	if cmd.ScopeID != "" {
		c.add(p+"/scope_id", CodeInvalid, "A manifest never sets the target of a command ("+ruleCommands+").")
	}
	options := map[string]bool{}
	for j, o := range cmd.Options {
		op := p + "/options/" + strconv.Itoa(j)
		if options[o.Name] {
			c.add(op+"/name", CodeInvalid, "The option name is used twice in this command ("+ruleCommands+").")
		}
		options[o.Name] = true
		c.text(op+"/description", o.Description, true)
	}
}

// lists checks components and incoming webhooks: no duplicates, texts through textcheck.
func (c *checker) lists(m Manifest) {
	seen := map[string]bool{}
	for i, k := range m.Components {
		if seen[k] {
			c.add("/components/"+strconv.Itoa(i), CodeInvalid, "The component type is listed twice ("+ruleFormat+").")
		}
		seen[k] = true
	}
	names := map[string]bool{}
	for i, w := range m.IncomingWebhooks {
		p := "/incoming_webhooks/" + strconv.Itoa(i)
		c.text(p+"/name", w.Name, true)
		if names[w.Name] {
			c.add(p+"/name", CodeInvalid, "The incoming webhook name is used twice ("+ruleFormat+").")
		}
		names[w.Name] = true
		c.text(p+"/purpose", w.Purpose, true)
	}
}

// HostErrorsOnly reports whether every error of fe is a host error, so installApp and
// upgradeAppManifest answer 400 webhook_host_not_allowed instead of validation_failed (M2-24
// design note 3 d). It is false for an empty list.
func HostErrorsOnly(fe []errs.FieldError) bool {
	if len(fe) == 0 {
		return false
	}
	for _, f := range fe {
		if f.Code != CodeHostNotAllowed {
			return false
		}
	}
	return true
}

// storableJSON reports whether the JSON text raw is free of the escapes PostgreSQL refuses in
// jsonb: \u0000 and a \u escape of a surrogate that is not part of a pair. encoding/json turns
// them into NUL and U+FFFD and lets them through, so without this rule a manifest that passes
// every check would fail in the database (SEC-IN-4, SEC-T06). raw is valid JSON when it is called.
func storableJSON(raw []byte) bool {
	sc := escapeScan{}
	for i := 0; i < len(raw); i++ {
		next, ok := sc.step(raw, i)
		if !ok {
			return false
		}
		i = next
	}
	return !sc.pendingHigh
}

// escapeScan is the state of storableJSON: inside a string or not, and whether the last escape
// was a high surrogate that waits for its low half.
type escapeScan struct {
	inString, pendingHigh bool
}

// step reads the byte at i of raw and returns the index of the last byte it consumed, and false
// when the text is not storable.
func (sc *escapeScan) step(raw []byte, i int) (int, bool) {
	b := raw[i]
	switch {
	case !sc.inString:
		sc.inString = b == '"'
		return i, true
	case b != '\\':
		sc.inString = b != '"'
		return i, !sc.pendingHigh
	case i+1 >= len(raw) || raw[i+1] != 'u':
		return i + 1, !sc.pendingHigh
	}
	cu, ok := hex4(raw[i+2:])
	return i + 5, ok && sc.unit(cu)
}

// unit takes the code unit cu of one \u escape and reports whether the text is still storable.
func (sc *escapeScan) unit(cu uint16) bool {
	switch {
	case cu == 0:
		return false
	case cu >= 0xDC00 && cu <= 0xDFFF:
		ok := sc.pendingHigh
		sc.pendingHigh = false
		return ok
	case sc.pendingHigh:
		return false
	}
	sc.pendingHigh = cu >= 0xD800 && cu <= 0xDBFF
	return true
}

// hex4 parses the four hex digits at the start of b.
func hex4(b []byte) (uint16, bool) {
	if len(b) < 4 {
		return 0, false
	}
	var v uint16
	for _, d := range b[:4] {
		switch {
		case d >= '0' && d <= '9':
			d -= '0'
		case d >= 'a' && d <= 'f':
			d -= 'a' - 10
		case d >= 'A' && d <= 'F':
			d -= 'A' - 10
		default:
			return 0, false
		}
		v = v<<4 | uint16(d)
	}
	return v, true
}
