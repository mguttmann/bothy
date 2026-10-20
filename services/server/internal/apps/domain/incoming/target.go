// SPDX-License-Identifier: Apache-2.0

package incoming

import (
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ThreadFact is what chat knows about a thread named by the webhook or the body.
type ThreadFact struct {
	// ID is the thread.
	ID ids.ID
	// ChannelID is the channel the thread belongs to now (a moved thread has the new one).
	ChannelID ids.ID
	// Found is false for an unknown ID.
	Found bool
}

// TargetFacts are the facts ChooseTarget decides on (M2-27 design note 5).
type TargetFacts struct {
	// Format is the classified format of the body.
	Format Format
	// ChannelID is the webhook's channel.
	ChannelID ids.ID
	// ChannelKind is "text" or "forum"; any other kind is treated like a text channel without a
	// main thread.
	ChannelKind string
	// MainThreadID is the main thread of a text channel, zero for a forum.
	MainThreadID ids.ID
	// Fixed is the fixed thread of the webhook, nil when it has none.
	Fixed *ThreadFact
	// Body is the thread_id of a native body, nil when absent.
	Body *ThreadFact
	// Title is the title of a native body, nil when absent.
	Title *string
}

// TargetKind says how the message is written.
type TargetKind int

// The kinds of target.
const (
	// TargetExisting writes into ThreadID (fixed thread, body thread or main thread).
	TargetExisting TargetKind = iota + 1
	// TargetForumPost creates a forum post with Title and the message as its first message.
	TargetForumPost
	// TargetTextThread writes the message into ThreadID, the main thread, then starts a new
	// thread with Title from it.
	TargetTextThread
)

// Target is the chosen target of a post.
type Target struct {
	// Kind says how the message is written.
	Kind TargetKind
	// ThreadID is the existing thread, the main thread for TargetTextThread and zero for
	// TargetForumPost.
	ThreadID ids.ID
	// Title is the title of the new thread, empty for TargetExisting.
	Title string
}

// ChooseTarget applies the rules of 11 section 8.3 with the rulings of M2-27 design note 5, in
// order: 1 the fixed thread while it is found in the webhook's channel, else 404 not_found (a
// webhook writes only into its own channel) and the body is ignored; 2 the body thread when it is
// found in the channel, otherwise silently the next rule, so nothing about other channels shows;
// 3 a title gives a forum post in a forum and a thread started from a main-thread message in a
// text channel; 4 the main thread of a text channel. A forum, or a channel without a main
// thread, reached by none of them gives 400 validation_failed with code required_for_forum_post,
// pointer /title for native and an empty pointer for alertmanager. Rules 2 and 3 apply to native
// bodies only.
func ChooseTarget(f TargetFacts) (Target, error) {
	if f.Fixed != nil {
		if !f.Fixed.Found || f.Fixed.ChannelID != f.ChannelID {
			return Target{}, errs.New(errs.CodeNotFound, "")
		}
		return Target{Kind: TargetExisting, ThreadID: f.Fixed.ID}, nil
	}
	native := f.Format == FormatNative
	if native && f.Body != nil && f.Body.Found && f.Body.ChannelID == f.ChannelID {
		return Target{Kind: TargetExisting, ThreadID: f.Body.ID}, nil
	}
	text := f.ChannelKind == "text" && !f.MainThreadID.IsZero()
	if native && f.Title != nil {
		switch {
		case f.ChannelKind == "forum":
			return Target{Kind: TargetForumPost, Title: *f.Title}, nil
		case text:
			return Target{Kind: TargetTextThread, ThreadID: f.MainThreadID, Title: *f.Title}, nil
		}
	}
	if text {
		return Target{Kind: TargetExisting, ThreadID: f.MainThreadID}, nil
	}
	pointer := ""
	if native {
		pointer = "/title"
	}
	return Target{}, fieldError(pointer, CodeRequiredForForumPost, "A post into a forum needs a title or a fixed thread (11 section 8.3).")
}
