// SPDX-License-Identifier: Apache-2.0

package incoming

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Payload is the Alertmanager webhook payload (version 4) as decoded from JSON. The maps hold the
// decoded JSON values and are nil when the member is absent; ParseAlertmanager checks them.
type Payload struct {
	// Version is the payload version; only "4" exists.
	Version string `json:"version"`
	// GroupKey identifies the alert group; it is not rendered.
	GroupKey string `json:"groupKey"`
	// TruncatedAlerts counts the alerts Alertmanager left out of Alerts.
	TruncatedAlerts int64 `json:"truncatedAlerts"`
	// Status is firing or resolved.
	Status string `json:"status"`
	// Receiver is the name of the receiver; it is not rendered.
	Receiver string `json:"receiver"`
	// GroupLabels are the labels the group is formed by.
	GroupLabels map[string]any `json:"groupLabels"`
	// CommonLabels are the labels all alerts share.
	CommonLabels map[string]any `json:"commonLabels"`
	// CommonAnnotations are the annotations all alerts share.
	CommonAnnotations map[string]any `json:"commonAnnotations"`
	// ExternalURL is the address of the sending Alertmanager.
	ExternalURL string `json:"externalURL"`
	// Alerts are the alerts of the notification.
	Alerts []RawAlert `json:"alerts"`
}

// RawAlert is one alert of a Payload as decoded from JSON.
type RawAlert struct {
	// Status is firing or resolved.
	Status string `json:"status"`
	// Labels identify the alert; nil when absent, which ParseAlertmanager refuses.
	Labels map[string]any `json:"labels"`
	// Annotations carry summary, description and runbook_url.
	Annotations map[string]any `json:"annotations"`
	// StartsAt is when the alert started; it is not rendered.
	StartsAt string `json:"startsAt"`
	// EndsAt is when the alert ended; it is not rendered.
	EndsAt string `json:"endsAt"`
	// GeneratorURL links to the source of the alert.
	GeneratorURL string `json:"generatorURL"`
	// Fingerprint identifies the alert; it is not rendered.
	Fingerprint string `json:"fingerprint"`
}

// Notification is a checked payload with string maps that are never nil.
type Notification struct {
	// Status is firing or resolved.
	Status string
	// TruncatedAlerts is not negative.
	TruncatedAlerts int64
	// GroupLabels, CommonLabels and CommonAnnotations are the maps of the payload.
	GroupLabels, CommonLabels, CommonAnnotations map[string]string
	// ExternalURL is the address of the sending Alertmanager, unchecked.
	ExternalURL string
	// Alerts holds at least one alert.
	Alerts []Alert
}

// Alert is one checked alert.
type Alert struct {
	// Status is firing or resolved.
	Status string
	// Labels and Annotations are never nil.
	Labels, Annotations map[string]string
	// GeneratorURL is the source address, unchecked.
	GeneratorURL string
}

// parseDetail is the detail of a refused payload; it names the rule, never a value.
const parseDetail = "The Alertmanager payload violates the rules of 11 section 8.2."

// statusValid reports the two statuses of the payload.
func statusValid(s string) bool { return s == "firing" || s == "resolved" }

// payloadChecker collects the field errors of one payload.
type payloadChecker struct{ fields []errs.FieldError }

// add appends one error at pointer.
func (c *payloadChecker) add(pointer, code, detail string) {
	c.fields = append(c.fields, errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// status checks a status member at pointer.
func (c *payloadChecker) status(pointer, s string) {
	switch {
	case s == "":
		c.add(pointer, CodeRequired, "The status is required.")
	case !statusValid(s):
		c.add(pointer, codeInvalidEnum, "The status is firing or resolved.")
	}
}

// strings converts a decoded JSON object at pointer into a string map; a value that is not a
// JSON string is an invalid_type error at its member. A nil map gives an empty map.
func (c *payloadChecker) strings(pointer string, m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		s, ok := m[k].(string)
		if !ok {
			c.add(pointer+"/"+escapePointer(k), CodeInvalidType, "Every label and annotation value is a string.")
			continue
		}
		out[k] = s
	}
	return out
}

// escapePointer escapes a member name for a JSON Pointer (RFC 6901 section 3).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// ParseAlertmanager checks an Alertmanager payload (M2-27 design note 4d) and returns it with
// string maps. The schema of the HTTP layer already enforces version 4, the enums and at least
// one alert; the function checks them again so it is total. Each alert needs status and labels
// (/alerts/<i>/labels, required), and every value in groupLabels, commonLabels,
// commonAnnotations, labels and annotations must be a JSON string (pointer to the member,
// invalid_type). Errors are one 400 validation_failed listing every failed field.
func ParseAlertmanager(p Payload) (Notification, error) {
	c := &payloadChecker{}
	switch p.Version {
	case "4":
	case "":
		c.add("/version", CodeRequired, "The version is required.")
	default:
		c.add("/version", codeInvalidEnum, "Only version 4 exists.")
	}
	c.status("/status", p.Status)
	if p.TruncatedAlerts < 0 {
		c.add("/truncatedAlerts", codeTooSmall, "truncatedAlerts is not negative.")
	}
	n := Notification{
		Status:            p.Status,
		TruncatedAlerts:   p.TruncatedAlerts,
		GroupLabels:       c.strings("/groupLabels", p.GroupLabels),
		CommonLabels:      c.strings("/commonLabels", p.CommonLabels),
		CommonAnnotations: c.strings("/commonAnnotations", p.CommonAnnotations),
		ExternalURL:       p.ExternalURL,
	}
	switch {
	case p.Alerts == nil:
		c.add("/alerts", CodeRequired, "alerts is required.")
	case len(p.Alerts) == 0:
		c.add("/alerts", codeTooFewItems, "alerts holds at least one alert.")
	}
	for i, a := range p.Alerts {
		base := "/alerts/" + strconv.Itoa(i)
		c.status(base+"/status", a.Status)
		if a.Labels == nil {
			c.add(base+"/labels", CodeRequired, "Every alert has labels.")
		}
		n.Alerts = append(n.Alerts, Alert{
			Status:       a.Status,
			Labels:       c.strings(base+"/labels", a.Labels),
			Annotations:  c.strings(base+"/annotations", a.Annotations),
			GeneratorURL: a.GeneratorURL,
		})
	}
	if len(c.fields) > 0 {
		return Notification{}, errs.New(errs.CodeValidationFailed, parseDetail).WithFields(c.fields...)
	}
	return n, nil
}

// RenderAlertmanager renders n as one message in the layout of M2-27 design note 7: a header
// line with status, title and count, one list item per alert with severity, summary, runbook and
// source, an item counting truncated or dropped alerts, and the Alertmanager address. Payload
// values have control characters replaced by a space and every ASCII punctuation character
// backslash-escaped, so a label adds no formatting, link or image; a URL appears only when it
// is an absolute http or https URL with a host, as an autolink. The result never exceeds
// maxBytes bytes (message.max_content_bytes) and is always valid UTF-8: alert items are dropped
// from the end and counted, then the footer goes, then the rest is cut at a UTF-8 boundary and
// ends in "...".
//
// Example output:
//
//	**FIRING** HighErrorRate (1 alert)
//
//	- **firing** HighErrorRate, severity critical: Error rate above 5 percent
//	  Source: <https://prometheus.example.com/graph?g0.expr=up>
//
//	Alertmanager: <https://alertmanager.example.com>
func RenderAlertmanager(n Notification, maxBytes int) string {
	title := n.CommonLabels["alertname"]
	if title == "" {
		title = n.GroupLabels["alertname"]
	}
	if title == "" {
		title = "Alerts"
	}
	total := uint64(len(n.Alerts)) + uint64(max(n.TruncatedAlerts, 0))
	count := strconv.FormatUint(total, 10) + " alerts"
	if total == 1 {
		count = "1 alert"
	}
	header := "**" + escapeValue(strings.ToUpper(n.Status)) + "** " + escapeValue(title) + " (" + count + ")"
	items := make([]string, len(n.Alerts))
	for i, a := range n.Alerts {
		items[i] = renderAlert(a, title)
	}
	footer := ""
	if u, ok := checkedURL(n.ExternalURL); ok {
		footer = "\n\nAlertmanager: <" + u + ">"
	}
	truncated := uint64(max(n.TruncatedAlerts, 0))
	for k := len(items); k >= 0; k-- {
		if s := assemble(header, items[:k], truncated+uint64(len(items)-k), footer); len(s) <= maxBytes {
			return s
		}
	}
	s := assemble(header, nil, truncated+uint64(len(items)), "")
	if len(s) <= maxBytes {
		return s
	}
	return cut(s, maxBytes)
}

// assemble joins the header, the kept items, the item counting more alerts and the footer.
func assemble(header string, items []string, more uint64, footer string) string {
	parts := slices.Clone(items)
	if more > 0 {
		parts = append(parts, "- and "+strconv.FormatUint(more, 10)+" more alerts")
	}
	s := header
	if len(parts) > 0 {
		s += "\n\n" + strings.Join(parts, "\n")
	}
	return s + footer
}

// renderAlert renders one list item with its continuation lines; title is the unescaped title
// of the notification, used when the alert has no alertname.
func renderAlert(a Alert, title string) string {
	name := a.Labels["alertname"]
	if name == "" {
		name = title
	}
	var b strings.Builder
	b.WriteString("- **" + escapeValue(a.Status) + "** " + escapeValue(name))
	if sev := a.Labels["severity"]; sev != "" {
		b.WriteString(", severity " + escapeValue(sev))
	}
	b.WriteString(": " + summary(a))
	if u, ok := checkedURL(a.Annotations["runbook_url"]); ok {
		b.WriteString("\n  Runbook: <" + u + ">")
	}
	if u, ok := checkedURL(a.GeneratorURL); ok {
		b.WriteString("\n  Source: <" + u + ">")
	}
	return b.String()
}

// summary is the annotation summary, else description, else the labels other than alertname and
// severity as name=value sorted by name, else "no summary"; payload values come escaped.
func summary(a Alert) string {
	if s := a.Annotations["summary"]; s != "" {
		return escapeValue(s)
	}
	if s := a.Annotations["description"]; s != "" {
		return escapeValue(s)
	}
	var names []string
	for k := range a.Labels {
		if k != "alertname" && k != "severity" {
			names = append(names, k)
		}
	}
	if len(names) == 0 {
		return "no summary"
	}
	slices.Sort(names)
	pairs := make([]string, len(names))
	for i, k := range names {
		pairs[i] = escapeValue(k) + "=" + escapeValue(a.Labels[k])
	}
	return strings.Join(pairs, ", ")
}

// escapeValue makes a payload value inert Markdown (design note 7e): invalid UTF-8 becomes
// U+FFFD, every control character one space, and every ASCII punctuation character gets a
// backslash (CommonMark backslash escapes), so no value opens emphasis, a link, an image, a
// mention or a line of its own.
func escapeValue(s string) string {
	var b strings.Builder
	for _, r := range strings.ToValidUTF8(s, "�") {
		switch {
		case unicode.IsControl(r):
			b.WriteByte(' ')
		case r < utf8.RuneSelf && (unicode.IsPunct(r) || unicode.IsSymbol(r)):
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// checkedURL returns s when it is an absolute http or https URL with a host, valid UTF-8 and
// without white space, control characters, '<' or '>', so it can stand in an autolink whose
// visible text is the target itself (SEC-OUT-1). The server never fetches it (SEC-OUT-2).
func checkedURL(s string) (string, bool) {
	if s == "" || !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == '<' || r == '>'
	}) {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return s, true
}

// cut shortens s to at most maxBytes bytes at a UTF-8 boundary, ending in "..." when it fits.
func cut(s string, maxBytes int) string {
	const ellipsis = "..."
	if maxBytes <= 0 {
		return ""
	}
	if maxBytes < len(ellipsis) {
		return ellipsis[:maxBytes]
	}
	end := maxBytes - len(ellipsis)
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + ellipsis
}
