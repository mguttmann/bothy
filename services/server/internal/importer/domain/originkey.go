// SPDX-License-Identifier: Apache-2.0

package domain

import "strings"

// Item kinds (03 import_items.item_kind).
const (
	KindSpace    = "space"
	KindChannel  = "channel"
	KindForumTag = "forum_tag"
	KindThread   = "thread"
	KindMessage  = "message"
	KindFile     = "file"
	KindUser     = "user"
	KindReaction = "reaction"
	KindPin      = "pin"
)

// Structure reports the item kinds this ticket writes: spaces, channels, forum tags and people.
// thread, message, pin and file items come with M8-11.
func Structure(kind string) bool {
	switch kind {
	case KindSpace, KindChannel, KindForumTag, KindUser:
		return true
	}
	return false
}

// OriginKey is a parsed origin key (22 7.1): its kind and its Discord IDs.
type OriginKey struct {
	Kind string
	// IDs are the Discord IDs of the key: one for space, channel and user; the channel and the
	// tag for forum_tag (a tag ID is unique only within its forum in the data the tool reads).
	IDs []string
}

// ParseOriginKey checks key against itemKind: space:<digits>, channel:<digits>,
// forum_tag:<channel digits>:<tag digits>, user:<digits>. It returns the field code of the
// violation: FieldKindMismatch for a prefix that is not itemKind, FieldInvalid for any other
// form. The column is text; the request body limit of the HTTP layer is its only length bound.
//
// Example:
//
//	k, code := ParseOriginKey("forum_tag", "forum_tag:1029384000:77") // k.IDs == ["1029384000", "77"], code == ""
func ParseOriginKey(itemKind, key string) (OriginKey, string) {
	prefix, rest, ok := strings.Cut(key, ":")
	if !ok {
		return OriginKey{}, FieldInvalid
	}
	if prefix != itemKind {
		return OriginKey{}, FieldKindMismatch
	}
	parts := strings.Split(rest, ":")
	want := 1
	if itemKind == KindForumTag {
		want = 2
	}
	if len(parts) != want {
		return OriginKey{}, FieldInvalid
	}
	for _, p := range parts {
		if !Digits(p) {
			return OriginKey{}, FieldInvalid
		}
	}
	return OriginKey{Kind: itemKind, IDs: parts}, ""
}

// Key returns the origin key of kind with ids, the inverse of ParseOriginKey.
func Key(kind string, ids ...string) string {
	return kind + ":" + strings.Join(ids, ":")
}
