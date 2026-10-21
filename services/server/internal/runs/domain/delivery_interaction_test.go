// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestInvocationPayload_Interaction: the payload of agent.invocation for a run an interaction
// opened has trigger_kind interaction and interaction_id, no message_id and no values (M2-15
// design note 11 c); an interaction trigger with a message, a message trigger with an
// interaction and an unknown kind are refused.
func TestInvocationPayload_Interaction(t *testing.T) {
	t.Parallel()
	ix := ids.New()
	in := domain.InvocationPayloadInput{
		RunID: ids.New(), ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), TriggerKind: "interaction", InteractionID: ix,
		TriggeredBy: domain.PrincipalRefPayload{ID: ids.New().String(), Kind: "user", Handle: "alex", DisplayName: "Alex"},
		BillingMode: "organization",
	}
	p, err := domain.InvocationPayload(in)
	if err != nil {
		t.Fatal(err)
	}
	validateAgainst(t, "AgentInvocationPayload", p)
	var got map[string]any
	if err := json.Unmarshal(p, &got); err != nil {
		t.Fatal(err)
	}
	if got["trigger_kind"] != "interaction" || got["interaction_id"] != ix.String() || got["message_id"] != nil {
		t.Fatalf("payload %s", p)
	}
	if id, err := domain.InvocationInteractionID(p); err != nil || id != ix {
		t.Fatalf("InvocationInteractionID = %v, %v", id, err)
	}
	for name, bad := range map[string]func(domain.InvocationPayloadInput) domain.InvocationPayloadInput{
		"with a message": func(v domain.InvocationPayloadInput) domain.InvocationPayloadInput { v.MessageID = ids.New(); return v },
		"without the object": func(v domain.InvocationPayloadInput) domain.InvocationPayloadInput {
			v.InteractionID = ids.ID{}
			return v
		},
		"an unknown kind": func(v domain.InvocationPayloadInput) domain.InvocationPayloadInput {
			v.TriggerKind = "command"
			return v
		},
		"message with object": func(v domain.InvocationPayloadInput) domain.InvocationPayloadInput {
			v.TriggerKind = "message"
			return v
		},
	} {
		if _, err := domain.InvocationPayload(bad(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if strings.Contains(string(p), "values") {
		t.Errorf("payload carries values: %s", p)
	}
}

// TestInteractionDeliveryIDs: the delivery of interaction.created reads the interaction and its
// run from the payload; a payload without a run or with a bad ID is refused.
func TestInteractionDeliveryIDs(t *testing.T) {
	t.Parallel()
	ix, run := ids.New(), ids.New()
	gotIx, gotRun, err := domain.InteractionDeliveryIDs([]byte(`{"interaction":{"id":"` + ix.String() + `","run_id":"` + run.String() + `","values":{}}}`))
	if err != nil || gotIx != ix || gotRun != run {
		t.Fatalf("InteractionDeliveryIDs = %v %v %v", gotIx, gotRun, err)
	}
	for _, bad := range []string{`{`, `{"interaction":{"id":"` + ix.String() + `"}}`, `{"interaction":{"id":"x","run_id":"` + run.String() + `"}}`} {
		if _, _, err := domain.InteractionDeliveryIDs([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", bad)
		}
	}
}
