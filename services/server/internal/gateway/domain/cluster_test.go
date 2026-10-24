// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fullSignals returns one signal of every kind with every member it may carry.
func fullSignals() []domain.Signal {
	n := ids.New()
	return []domain.Signal{
		{Kind: domain.SignalTyping, Node: n, ThreadID: ids.New(), PrincipalID: ids.New()},
		{Kind: domain.SignalRun, Node: n, RunID: ids.New(), ThreadID: ids.New(), Version: 9007199254740991},
		{Kind: domain.SignalAgentPresence, Node: n, AgentID: ids.New(), State: "online", BillingMode: "subscription", ActorID: ids.New()},
		{Kind: domain.SignalAgentPresence, Node: n, AgentID: ids.New(), State: "offline"},
		{Kind: domain.SignalDevice, Node: n, DeviceID: ids.New(), SessionID: ids.New(), BindID: ids.New()},
		{Kind: domain.SignalConsumer, Node: n, AgentID: ids.New(), SessionID: ids.New()},
		{Kind: domain.SignalHint, Node: n, RecipientID: ids.New(), NotificationID: ids.New(), HintKind: "input_needed", ThreadID: ids.New(),
			MessageID: ids.New(), TargetID: ids.New(), Quiet: true, Count: 9007199254740991},
		{Kind: domain.SignalHint, Node: n, RecipientID: ids.New(), NotificationID: ids.New(), HintKind: "security", TargetID: ids.New()},
	}
}

// TestSignal_RoundTrip: every kind encodes to its members only and decodes to the same signal; the
// largest one stays below the NOTIFY limit.
func TestSignal_RoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range fullSignals() {
		b, err := domain.EncodeSignal(s)
		if err != nil {
			t.Fatalf("%s: %v", s.Kind, err)
		}
		if len(b) >= domain.NotifyPayloadMax/10 {
			t.Fatalf("%s: %d bytes", s.Kind, len(b))
		}
		got, ours, err := domain.DecodeSignal(b)
		if err != nil || !ours {
			t.Fatalf("%s: ours %v, %v (%s)", s.Kind, ours, err, b)
		}
		if got != s {
			t.Fatalf("%s: got %+v, want %+v", s.Kind, got, s)
		}
	}
}

// TestSignal_HintCarriesQuietAndCount: a hint always names quiet and count, also when they are
// false and zero.
func TestSignal_HintCarriesQuietAndCount(t *testing.T) {
	t.Parallel()
	b, err := domain.EncodeSignal(domain.Signal{Kind: domain.SignalHint, Node: ids.New(), RecipientID: ids.New(), NotificationID: ids.New(),
		HintKind: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"quiet":false`) || !strings.Contains(string(b), `"count":0`) {
		t.Fatalf("%s", b)
	}
}

// TestSignal_EncodeRefuses: a signal outside the shape of its kind does not encode.
func TestSignal_EncodeRefuses(t *testing.T) {
	t.Parallel()
	n := ids.New()
	for name, s := range map[string]domain.Signal{
		"unknown kind":     {Kind: "gateway.other", Node: n},
		"no node":          {Kind: domain.SignalTyping, ThreadID: ids.New(), PrincipalID: ids.New()},
		"missing member":   {Kind: domain.SignalTyping, Node: n, ThreadID: ids.New()},
		"foreign member":   {Kind: domain.SignalTyping, Node: n, ThreadID: ids.New(), PrincipalID: ids.New(), RunID: ids.New()},
		"text as a code":   {Kind: domain.SignalAgentPresence, Node: n, AgentID: ids.New(), State: "Alex is typing"},
		"negative version": {Kind: domain.SignalRun, Node: n, RunID: ids.New(), ThreadID: ids.New(), Version: -1},
		"negative count":   {Kind: domain.SignalHint, Node: n, RecipientID: ids.New(), NotificationID: ids.New(), HintKind: "message", Count: -1},
		"quiet on typing":  {Kind: domain.SignalTyping, Node: n, ThreadID: ids.New(), PrincipalID: ids.New(), Quiet: true},
	} {
		if _, err := domain.EncodeSignal(s); !errors.Is(err, domain.ErrMalformedSignal) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestSignal_DecodeForeignAndMalformed: a terminal kind is foreign; a gateway kind with a wrong,
// missing, extra or repeated member is malformed; no kind at all is malformed.
func TestSignal_DecodeForeignAndMalformed(t *testing.T) {
	t.Parallel()
	if _, ours, err := domain.DecodeSignal([]byte(`{"kind":"terminal.host_close","host_id":"x"}`)); ours || err != nil {
		t.Fatalf("foreign: ours %v, %v", ours, err)
	}
	n, th, p := ids.New(), ids.New(), ids.New()
	good := func(extra string) string {
		return `{"kind":"gateway.typing","node":"` + n.String() + `","thread_id":"` + th.String() + `","principal_id":"` + p.String() + `"` + extra + `}`
	}
	if _, ours, err := domain.DecodeSignal([]byte(good(""))); !ours || err != nil {
		t.Fatalf("good: %v %v", ours, err)
	}
	for name, payload := range map[string]string{
		"not json":       `typing.started:` + th.String(),
		"no kind":        `{"node":"` + n.String() + `"}`,
		"unknown kind":   `{"kind":"gateway.other","node":"` + n.String() + `"}`,
		"extra member":   good(`,"run_id":"` + ids.New().String() + `"`),
		"repeated":       good(`,"thread_id":"` + th.String() + `"`),
		"upper case id":  strings.Replace(good(""), th.String(), strings.ToUpper(th.String()), 1),
		"missing member": `{"kind":"gateway.typing","node":"` + n.String() + `","thread_id":"` + th.String() + `"}`,
		"no node":        `{"kind":"gateway.typing","thread_id":"` + th.String() + `","principal_id":"` + p.String() + `"}`,
		"trailing":       good("") + `{}`,
		"fraction":       `{"kind":"gateway.run","node":"` + n.String() + `","run_id":"` + th.String() + `","thread_id":"` + p.String() + `","version":1.5}`,
		"zero version":   `{"kind":"gateway.run","node":"` + n.String() + `","run_id":"` + th.String() + `","thread_id":"` + p.String() + `","version":0}`,
		"text code": `{"kind":"gateway.agent_presence","node":"` + n.String() + `","agent_id":"` + th.String() +
			`","state":"see you at 5"}`,
	} {
		if _, ours, err := domain.DecodeSignal([]byte(payload)); !errors.Is(err, domain.ErrMalformedSignal) || (name != "not json" && name != "no kind" && name != "trailing" && !ours) {
			t.Errorf("%s: ours %v, %v", name, ours, err)
		}
	}
}

// TestSignal_CarriesNoContent: the encoded members of every kind are IDs, numbers, booleans and
// codes; no member can hold free text (03 section 6.6).
func TestSignal_CarriesNoContent(t *testing.T) {
	t.Parallel()
	for _, s := range fullSignals() {
		b, err := domain.EncodeSignal(s)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for k, v := range m {
			text, isText := v.(string)
			if !isText || k == "kind" {
				continue
			}
			if _, err := ids.Parse(text); err != nil && strings.ContainsAny(text, " ABCDEFGHIJKLMNOPQRSTUVWXYZ.") {
				t.Errorf("%s: member %s carries %q", s.Kind, k, text)
			}
		}
	}
}
