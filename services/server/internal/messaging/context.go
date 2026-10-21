// SPDX-License-Identifier: Apache-2.0

package messaging

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// ContextQuery selects messages of one thread for a run context (docs/spec/09-agenten.md section
// 4.6; M2-13 design note 3). The caller (runs) authorized run.context.read on the run of this
// thread in the same request; every returned item passes message.read in filter mode as well.
type ContextQuery struct {
	ThreadID ids.ID
	// AfterSeq is the exclusive lower bound (context_floor_seq, or 0).
	AfterSeq int64
	// BeforeSeq is the exclusive upper bound (thread_seq of the triggering message or of the
	// parent message).
	BeforeSeq int64
	// Limit keeps the newest Limit selectable messages (history_backfill_limit); 0 returns none.
	Limit int
	// ExcludeAuthorID leaves out the messages of this author (the agent itself in session mode
	// resume); zero leaves out none. The exclusion applies before Limit.
	ExcludeAuthorID ids.ID
}

// ContextPage holds the selected messages ascending by thread_seq with their full content.
type ContextPage struct {
	Items []Message
	// HasOlder reports a selectable message with AfterSeq < thread_seq < the first item (or <
	// BeforeSeq when Items is empty) that Limit left out.
	HasOlder bool
}
