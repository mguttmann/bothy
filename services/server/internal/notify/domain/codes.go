// SPDX-License-Identifier: Apache-2.0

package domain

// KindSync marks the badge sync of 13 section 8: a push without thread, message or text that only
// sets the badge number of the recipient's mobile devices. It is no NotificationKind, so
// KindSync.Valid is false; it exists only as the code 9 of the job notify.push_send
// (docs/DECISIONS.md D-M3-19-10, D-M3-19-15).
const KindSync Kind = "sync"

// codeSync is the fixed code of KindSync.
const codeSync int64 = 9

// KindCode returns the fixed code of k for the argument Kind of the job notify.push_send: 1 to 8
// for the first eight kinds in the order of NotificationKind, 9 for KindSync and from 10 on for the
// kinds added later in their order (wake_ended 10, AR-04). The codes are part of the stored job
// arguments and are never renumbered (D-M3-19-10), so a later kind skips the code of KindSync.
//
// Errors: an internal_error for any other kind.
//
// Example:
//
//	code, _ := domain.KindCode(domain.KindMention) // 2
func KindCode(k Kind) (int64, error) {
	if k == KindSync {
		return codeSync, nil
	}
	for i, known := range kinds {
		if known == k {
			code := int64(i + 1)
			if code >= codeSync {
				code++
			}
			return code, nil
		}
	}
	return 0, invalid("kind")
}

// KindFromCode is the inverse of KindCode.
//
// Errors: an internal_error for a code outside 1 to len(Kinds())+1.
func KindFromCode(code int64) (Kind, error) {
	if code == codeSync {
		return KindSync, nil
	}
	i := code - 1
	if code > codeSync {
		i = code - 2
	}
	if code < 1 || i >= int64(len(kinds)) {
		return "", invalid("kind_code")
	}
	return kinds[i], nil
}
