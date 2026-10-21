// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// fcmMaxDataBytes is the FCM limit of the data of a data message, 4096 bytes (13 6.2 [Q9]). It is
// Google's fixed limit, not an operator value; a message above it is never truncated.
const fcmMaxDataBytes = 4096

// The values of android.priority (13 5.2, 8).
const (
	priorityHigh   = "high"
	priorityNormal = "normal"
)

// payloadVersion is data.v, the version of the payload of 13 6.2.
const payloadVersion = "1"

// fcmData is message.data in the key order of 13 6.2: a struct with string fields, not a map,
// so no other key can appear (ADR 0031). Every value holds an ID, a number or a fixed key.
type fcmData struct {
	V       string `json:"v"`
	NID     string `json:"nid"`
	K       string `json:"k"`
	I       string `json:"i"`
	T       string `json:"t,omitempty"`
	M       string `json:"m,omitempty"`
	Badge   string `json:"badge"`
	Quiet   string `json:"quiet,omitempty"`
	Cleared string `json:"cleared,omitempty"`
}

// fcmAndroid is message.android: the priority and the lifetime, no notification and no
// collapse_key (13 5.2, R-PN-08, D-PN-06).
type fcmAndroid struct {
	Priority string `json:"priority"`
	TTL      string `json:"ttl"`
}

// fcmBody is message: the token, android and data; no notification, apns or webpush block.
type fcmBody struct {
	Token   string     `json:"token"`
	Android fcmAndroid `json:"android"`
	Data    fcmData    `json:"data"`
}

// fcmMessage is the request body of messages:send.
type fcmMessage struct {
	Message fcmBody `json:"message"`
}

// buildMessage serializes the message of n for token (Design note 5 of brief M3-21): a hint
// with v, nid, k, i, t and m when present, badge and quiet; the badge sync of 13 8 with v, nid,
// k = sync, i, badge and cleared, with priority normal. ttl is push.ttl, written in whole
// seconds. ok is false when data is above fcmMaxDataBytes; a sync then goes without cleared
// instead (complete or absent, never partial). inflate is a test hook that changes data before
// the size check; nil in production.
func buildMessage(token string, n app.Notification, ttl time.Duration, inflate func(*fcmData)) (body []byte, ok bool) {
	d := fcmData{V: payloadVersion, NID: n.ID.String(), K: n.Kind, I: n.InstanceID.String(), Badge: strconv.FormatInt(n.Badge, 10)}
	priority := priorityHigh
	if n.Kind == app.KindSync {
		priority = priorityNormal
		d.Cleared = joinIDs(n.Cleared)
	} else {
		d.T, d.M = optionalID(n.ThreadID), optionalID(n.MessageID)
		d.Quiet = "0"
		if n.Quiet {
			d.Quiet, priority = "1", priorityNormal
		}
	}
	if inflate != nil {
		inflate(&d)
	}
	if !fits(d) {
		if d.Cleared == "" {
			return nil, false
		}
		// The app keeps the cleared notifications until it opens, the limit 13 8 states for iOS.
		d.Cleared = ""
		if !fits(d) {
			return nil, false
		}
	}
	b, err := json.Marshal(fcmMessage{Message: fcmBody{Token: token,
		Android: fcmAndroid{Priority: priority, TTL: strconv.FormatInt(int64(ttl/time.Second), 10) + "s"}, Data: d}})
	if err != nil {
		return nil, false
	}
	return b, true
}

// fits reports whether the serialized data stays within fcmMaxDataBytes.
func fits(d fcmData) bool {
	b, err := json.Marshal(d)
	return err == nil && len(b) <= fcmMaxDataBytes
}

// optionalID returns the canonical form of id, or "" for the zero ID (the key is then absent).
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// joinIDs joins the IDs with commas in the order given; "" for none.
func joinIDs(list []ids.ID) string {
	parts := make([]string, len(list))
	for i, id := range list {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}
