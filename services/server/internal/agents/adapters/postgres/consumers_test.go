// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestConsumerCursorQueries holds the statements of agent_consumer_cursors (brief M2-07 notes 2
// and 10): the first take creates the row at the head, a later one keeps acked_seq and writes the
// holder; the save and the restart are fenced by the holder; the acknowledgement of
// ackAgentEvents is monotone without holder; the row ends with its agent.
func TestConsumerCursorQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	agent := e.principal(t, "agent", "support-agent")
	if _, err := e.owner.Exec(ctx, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent.String()); err != nil {
		t.Fatal(err)
	}
	key := agents.ConsumerKey{AgentID: agent, Consumer: "default"}
	first, second := ids.New(), ids.New()
	if _, found, err := e.repo.GetConsumer(ctx, key); err != nil || found {
		t.Fatalf("before the first take: %v %v", found, err)
	}
	acked, created, err := e.repo.TakeConsumer(ctx, key, first, 40)
	if err != nil || !created || acked != 40 {
		t.Fatalf("first take = %d %v %v", acked, created, err)
	}
	if found, err := e.repo.SaveConsumerAck(ctx, key, first, 45); err != nil || !found {
		t.Fatalf("save of the holder = %v %v", found, err)
	}
	if found, err := e.repo.SaveConsumerAck(ctx, key, first, 42); err != nil || !found {
		t.Fatalf("lower save = %v %v", found, err)
	}
	acked, created, err = e.repo.TakeConsumer(ctx, key, second, 90)
	if err != nil || created || acked != 45 {
		t.Fatalf("second take = %d %v %v", acked, created, err)
	}
	if found, err := e.repo.SaveConsumerAck(ctx, key, first, 50); err != nil || found {
		t.Fatalf("save of the replaced holder = %v %v", found, err)
	}
	if found, err := e.repo.RestartConsumer(ctx, key, first, 90); err != nil || found {
		t.Fatalf("restart of the replaced holder = %v %v", found, err)
	}
	if found, err := e.repo.RestartConsumer(ctx, key, second, 30); err != nil || !found {
		t.Fatalf("restart of the holder = %v %v", found, err)
	}
	if acked, found, err := e.repo.AckConsumer(ctx, key, 35); err != nil || !found || acked != 35 {
		t.Fatalf("ack = %d %v %v", acked, found, err)
	}
	if acked, found, err := e.repo.AckConsumer(ctx, key, 20); err != nil || !found || acked != 35 {
		t.Fatalf("lower ack = %d %v %v", acked, found, err)
	}
	if _, found, err := e.repo.AckConsumer(ctx, agents.ConsumerKey{AgentID: agent, Consumer: "other"}, 1); err != nil || found {
		t.Fatalf("ack of an unknown consumer = %v %v", found, err)
	}
	if acked, found, err := e.repo.GetConsumer(ctx, key); err != nil || !found || acked != 35 {
		t.Fatalf("get = %d %v %v", acked, found, err)
	}
	var holder string
	if err := e.owner.QueryRow(ctx, `SELECT holder_session_id::text FROM agent_consumer_cursors WHERE agent_id = $1`, agent.String()).Scan(&holder); err != nil || holder != second.String() {
		t.Fatalf("holder %q %v", holder, err)
	}
	if _, err := e.owner.Exec(ctx, `DELETE FROM agents WHERE principal_id = $1`, agent.String()); err != nil {
		t.Fatal(err)
	}
	if _, found, err := e.repo.GetConsumer(ctx, key); err != nil || found {
		t.Fatalf("the row outlived its agent: %v %v", found, err)
	}
}
