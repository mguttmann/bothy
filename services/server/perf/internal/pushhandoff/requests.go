// SPDX-License-Identifier: Apache-2.0

package pushhandoff

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
)

// Hint is the record of one request at Fake-APNs (design note 8 b). Token stays inside the test:
// it maps to a registration and is never printed.
type Hint struct {
	// Token is the device token of the path; APNsID and CollapseID the headers apns-id and
	// apns-collapse-id.
	Token, APNsID, CollapseID string
	// Sync is true for a body without b, the badge sync form of 13 section 8.
	Sync bool
	// NID, Kind, Instance, Thread and Message are b.nid, b.k, b.i, b.t and b.m.
	NID, Kind, Instance, Thread, Message string
	// Badge is aps.badge; nil when absent.
	Badge *int64
	// AcceptedAt is the moment the Fake accepted with 200; zero for any other answer.
	AcceptedAt time.Time
}

// hintBody is the body of 13 6.1; DecodeRequest rejects every member it does not name.
type hintBody struct {
	APS struct {
		Alert *struct {
			TitleLocKey string `json:"title-loc-key"`
			LocKey      string `json:"loc-key"`
		} `json:"alert"`
		MutableContent    int    `json:"mutable-content"`
		ThreadID          string `json:"thread-id"`
		Category          string `json:"category"`
		Sound             string `json:"sound"`
		Badge             *int64 `json:"badge"`
		InterruptionLevel string `json:"interruption-level"`
	} `json:"aps"`
	B *struct {
		V   int    `json:"v"`
		NID string `json:"nid"`
		K   string `json:"k"`
		I   string `json:"i"`
		T   string `json:"t"`
		M   string `json:"m"`
	} `json:"b"`
}

// DecodeRequest decodes a recorded request strictly into a hint record: the path token, the
// headers apns-id and apns-collapse-id, and the body {"aps": {...}, "b": {...}} of 13 6.1. A body
// without b is the badge sync form. The error reports a body that is not one JSON object of that
// shape, with an unknown member anywhere; it never contains the token.
func DecodeRequest(r apnstest.Request) (Hint, error) {
	var b hintBody
	dec := json.NewDecoder(bytes.NewReader(r.Body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return Hint{}, fmt.Errorf("pushhandoff: body: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Hint{}, errors.New("pushhandoff: body: data after the object")
	}
	h := Hint{Token: r.Token, APNsID: r.Header.Get("Apns-Id"), CollapseID: r.Header.Get("Apns-Collapse-Id"),
		Badge: b.APS.Badge, AcceptedAt: r.AcceptedAt, Sync: b.B == nil}
	if b.B != nil {
		h.NID, h.Kind, h.Instance, h.Thread, h.Message = b.B.NID, b.B.K, b.B.I, b.B.T, b.B.M
	}
	return h, nil
}
