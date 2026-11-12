// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The server checks imported values like any other input, and a batch is one transaction: one
// refused item loses the batch, the cursor stays, and every later run stops at the same place.
// Discord allows values the server refuses, so the run fits them before it sends them (names and
// topics, messages that convert to nothing) or follows the answer of the server where only the
// server knows the rule (the pins of a thread); the report counts or notes each case
// (docs/decisions.d/AUD-srv-cmd-01.md).

// TopicMaxChars is the length the run cuts a channel topic to: the default of
// channel.topic_max_chars of the server. getInstance does not carry that value, so the tool cannot
// read the effective one; a server that allows less refuses the channel item, and the run ends
// with exit 1 and names the topic. A test against the real server holds the number equal to the
// default.
const TopicMaxChars = 1024

// displayText fits a name or a topic of Discord to the display text rule of the server (SEC-IN-4,
// kernel/textcheck: no control character, line breaks and tabs included, no bidi control, valid
// UTF-8): every control character becomes a space, bidi controls and invalid bytes are dropped,
// and the white space this leaves at the ends is trimmed. changed is false, and s is returned as
// it is, when it already fits.
func displayText(s string) (fitted string, changed bool) {
	out := strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cc, r):
			return ' '
		case textcheck.IsBidiControl(r):
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	if out == s {
		return s, false
	}
	return strings.TrimSpace(out), true
}

// fitTopic returns the topic the run sends for a Discord topic: trimmed, fitted by displayText
// and cut to TopicMaxChars characters. changed reports a fit or a cut, not the trim.
func fitTopic(topic string) (fitted string, changed bool) {
	fitted, changed = displayText(strings.TrimSpace(topic))
	if utf8.RuneCountInString(fitted) > TopicMaxChars {
		fitted, changed = string([]rune(fitted)[:TopicMaxChars]), true
	}
	return fitted, changed
}

// visible reports a character that is no white space, which the server asks of the content of a
// message without a file.
func visible(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0
}

// forwardedAttachments returns the attachments of the messages m forwards (message_snapshots).
func forwardedAttachments(m discord.Message) []discord.Attachment {
	var out []discord.Attachment
	for _, s := range m.MessageSnapshots {
		out = append(out, s.Message.Attachments...)
	}
	return out
}

// hollow reports a message of a taken type that would reach the server with nothing: its converted
// text has no visible character and it has no attachment, neither its own nor one of a forward
// (an embed with only an image, a forward of such a message). The server refuses such content, so
// the message counts as skipped with SkipEmpty (22 6.1). The rule reads the message alone, so a
// later run that looks a reply target up decides the same.
func hollow(m discord.Message, text string) bool {
	return !visible(text) && len(m.Attachments) == 0 && len(forwardedAttachments(m)) == 0
}

// The refusal of a pin above pin.max_per_thread (messaging: ImportMessages.Pin): the field code
// and the rest of the pointer behind the item.
const (
	codeLimitReached = "limit_reached"
	pinMemberPointer = "/data/message_origin_key"
)

// dropPinsAtLimit handles the answer of the server to a pin above pin.max_per_thread (22 6.6):
// when err is that refusal of a pin item of b, it removes that pin item and every pin item after
// it from b (the thread is full, each of them would be refused too) and returns how many it
// removed; otherwise it returns 0 and leaves b alone. The refused batch wrote nothing, so the
// caller sends the smaller batch again.
func dropPinsAtLimit(err error, b *restclient.ImportBatch) int {
	var e *APIError
	if !errors.As(err, &e) || e.Status != http.StatusBadRequest {
		return 0
	}
	for _, f := range e.Fields {
		i, rest, ok := itemPointer(f.Pointer)
		if !ok || f.Code != codeLimitReached || rest != pinMemberPointer || i >= len(b.Items) || !isPin(b.Items[i]) {
			continue
		}
		kept := slices.Clone(b.Items[:i])
		dropped := 0
		for _, it := range b.Items[i:] {
			if isPin(it) {
				dropped++
				continue
			}
			kept = append(kept, it)
		}
		b.Items = kept
		return dropped
	}
	return 0
}

// isPin reports a pin item.
func isPin(o restclient.ImportObject) bool { return o.ItemKind == restclient.ImportObjectItemKindPin }
