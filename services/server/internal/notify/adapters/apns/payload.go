// SPDX-License-Identifier: Apache-2.0

package apns

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// appleMaxPayloadBytes is Apple's fixed limit of an APNs payload (13 6.1 [Q14]): Apple refuses a
// larger one with 413 PayloadTooLarge. It is a limit of the service, not an operator value.
const appleMaxPayloadBytes = 4096

// The fixed values of the payload (13 6.1).
const (
	titleLocKey    = "PUSH_TITLE"
	bodyLocPrefix  = "PUSH_BODY_"
	soundDefault   = "default"
	levelActive    = "active"
	levelPassive   = "passive"
	levelTimeSens  = "time-sensitive"
	payloadVersion = 1
	mutableContent = 1
)

// The headers of a request (13 5.1).
const (
	headerAuthorization = "Authorization"
	headerPushType      = "Apns-Push-Type"
	headerID            = "Apns-Id"
	headerTopic         = "Apns-Topic"
	headerPriority      = "Apns-Priority"
	headerExpiration    = "Apns-Expiration"
	headerCollapseID    = "Apns-Collapse-Id"
	headerContentType   = "Content-Type"
	headerUserAgent     = "User-Agent"
	pushTypeAlert       = "alert"
	priorityImmediate   = "10"
	priorityQuiet       = "5"
	contentTypeJSON     = "application/json"
)

// apsAlert is aps.alert: two keys of the app's localization files, never a text.
type apsAlert struct {
	TitleLocKey string `json:"title-loc-key"`
	LocKey      string `json:"loc-key"`
}

// aps is the object aps in the key order of 13 6.1. Badge has no omitempty: 0 clears the badge.
type aps struct {
	Alert             *apsAlert `json:"alert,omitempty"`
	MutableContent    int       `json:"mutable-content,omitempty"`
	ThreadID          string    `json:"thread-id,omitempty"`
	Category          string    `json:"category,omitempty"`
	Sound             string    `json:"sound,omitempty"`
	Badge             int64     `json:"badge"`
	InterruptionLevel string    `json:"interruption-level,omitempty"`
}

// ownKeys is the object b next to aps: the product's own keys, IDs and fixed values only.
type ownKeys struct {
	V   int    `json:"v"`
	NID string `json:"nid"`
	K   string `json:"k"`
	I   string `json:"i"`
	T   string `json:"t,omitempty"`
	M   string `json:"m,omitempty"`
}

// payload is the whole body; structs, not maps, so no other key can appear (ADR 0031).
type payload struct {
	APS aps      `json:"aps"`
	B   *ownKeys `json:"b,omitempty"`
}

// buildPayload returns the payload of n (Design note 5 a and d of brief M3-20). The caller has
// checked the kind.
func buildPayload(n app.Notification, timeSensitive bool) payload {
	if n.Kind == app.KindSync {
		return payload{APS: aps{Badge: n.Badge}}
	}
	kind := domain.Kind(n.Kind)
	upper := strings.ToUpper(n.Kind)
	p := payload{
		APS: aps{
			Alert:             &apsAlert{TitleLocKey: titleLocKey, LocKey: bodyLocPrefix + upper},
			MutableContent:    mutableContent,
			ThreadID:          idText(n.ThreadID),
			Category:          upper,
			Sound:             soundDefault,
			Badge:             n.Badge,
			InterruptionLevel: interruptionLevel(kind, n.Quiet, timeSensitive),
		},
		B: &ownKeys{V: payloadVersion, NID: n.ID.String(), K: n.Kind, I: n.InstanceID.String(), T: idText(n.ThreadID), M: idText(n.MessageID)},
	}
	if n.Quiet {
		p.APS.Sound = ""
	}
	return p
}

// interruptionLevel is passive when quiet, time-sensitive for input_needed with the switch, else
// active; never critical (13 6.1). Quiet wins, because 13 3.4 makes a hint in quiet hours quiet.
func interruptionLevel(k domain.Kind, quiet, timeSensitive bool) string {
	switch {
	case quiet:
		return levelPassive
	case k == domain.KindInputNeeded && timeSensitive:
		return levelTimeSens
	}
	return levelActive
}

// idText is the canonical form of id, or "" for the zero ID.
func idText(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// marshalPayload serializes p.
func marshalPayload(p payload) ([]byte, error) { return json.Marshal(p) }

// setHeaders writes exactly the headers of 13 5.1 for n and jwt onto h (Design note 5 b and d).
// User-Agent is set empty, so net/http sends none.
func setHeaders(h http.Header, n app.Notification, topic, jwt string, now time.Time, ttl time.Duration) {
	h.Set(headerAuthorization, "bearer "+jwt)
	h.Set(headerPushType, pushTypeAlert)
	h.Set(headerID, n.ID.String())
	h.Set(headerTopic, topic)
	priority := priorityImmediate
	if n.Quiet || n.Kind == app.KindSync {
		priority = priorityQuiet
	}
	h.Set(headerPriority, priority)
	h.Set(headerExpiration, unixSeconds(now.Add(ttl)))
	if n.Kind != app.KindSync && !n.ThreadID.IsZero() {
		h.Set(headerCollapseID, n.ThreadID.String())
	}
	h.Set(headerContentType, contentTypeJSON)
	h[headerUserAgent] = []string{""}
}
