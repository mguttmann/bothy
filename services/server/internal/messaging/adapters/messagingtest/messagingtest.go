// SPDX-License-Identifier: Apache-2.0

// Package messagingtest holds test helpers of the messaging module. Only _test.go files import
// it; TestMessagingtest_NotInBinary fails when the server binary depends on it.
//
// InsertImportedMessage places a message of origin import in a thread of an existing channel,
// the one state of a message that no operation writes outside an import source's own channels
// (05 rule 1.8, docs/spec/22): the agent conformance suite (M2-31, AG-20) needs such a message
// next to messages of people, agents and apps in one thread.
//
// Example:
//
//	id := messagingtest.InsertImportedMessage(t, pool, messagingtest.ImportedMessage{ThreadID: thread, AuthorID: sam, Content: "Imported note"})
package messagingtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Execer runs one statement: *pg.Pool, pgx.Tx and *pgx.Conn satisfy it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ImportedMessage is one message of origin import: its thread, its author and its text.
type ImportedMessage struct {
	ThreadID, AuthorID ids.ID
	Content            string
}

// InsertImportedMessage stores m as the import stores a message (03 messages, M8-11): origin
// import, kind text, silent, no mentions and no trigger_state. It takes the next thread_seq the
// way the chat port's append does (threads.last_message_seq and message_count raised in the
// same statement, so the sequence stays gapless) and publishes no MessagePosted, because an
// imported message never triggers an agent. It returns the new message ID and fails tb when the
// statement fails or the thread does not exist.
func InsertImportedMessage(tb testing.TB, db Execer, m ImportedMessage) ids.ID {
	tb.Helper()
	id := ids.New()
	tag, err := db.Exec(context.Background(), `WITH t AS (
			UPDATE threads SET last_message_seq = last_message_seq + 1, message_count = message_count + 1, last_activity_at = now()
			WHERE id = $1 RETURNING channel_id, last_message_seq)
		INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256,
			origin, kind, content_md, silent)
		SELECT $2, $1, t.channel_id, t.last_message_seq, $3, $2, '\x00', 'import', 'text', $4, true FROM t`,
		m.ThreadID.String(), id.String(), m.AuthorID.String(), m.Content)
	if err != nil {
		tb.Fatalf("messagingtest: storing an imported message: %v", err)
	}
	if tag.RowsAffected() != 1 {
		tb.Fatalf("messagingtest: storing an imported message: the thread %s does not exist", m.ThreadID)
	}
	return id
}
