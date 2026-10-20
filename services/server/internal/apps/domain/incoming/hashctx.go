// SPDX-License-Identifier: Apache-2.0

package incoming

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// HashContext returns the context part of the request hash of a native post (M2-27 design note
// 6c): "webhook:" + webhookID + "|thread:" + the body's thread ID or empty + "|title:" + the
// title or empty. The hash then covers the request as sent, and a client_msg_id repeated
// through another webhook of the same app is a reuse, not a replay of another webhook's message.
func HashContext(webhookID ids.ID, bodyThread *ids.ID, title *string) string {
	thread, t := "", ""
	if bodyThread != nil {
		thread = bodyThread.String()
	}
	if title != nil {
		t = *title
	}
	return "webhook:" + webhookID.String() + "|thread:" + thread + "|title:" + t
}
