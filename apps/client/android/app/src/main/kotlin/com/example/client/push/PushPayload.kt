// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

/**
 * The `data` of an FCM data message (docs/spec/13-push-notifications.md 6.2): only IDs, every
 * value a string.
 *
 * [parse] turns the map into a [PushHint] (a notification of a kind of 13 2) or a [PushSync]
 * (`k = "sync"`, 13 8); anything else is ignored, so a newer server's message never breaks the
 * service.
 */
sealed interface PushPayload {
    /** `v`, `nid` and `i`, which every message carries. */
    val nid: String

    /** The `instance_id` of the message. */
    val instanceId: String

    /** The badge number of 13 8, or null when the message carries none. */
    val badge: Int?

    companion object {
        /** The payload version this build understands. */
        const val VERSION = "1"

        /** The kind of the sync form of 13 8. */
        const val SYNC = "sync"

        /**
         * Reads [data], the `data` map of the message.
         *
         * @return the hint or the sync, or null when `v` is not `"1"` or a required key (`nid`,
         *   `k`, `i`) is missing or empty.
         */
        fun parse(data: Map<String, String>): PushPayload? {
            if (data["v"] != VERSION) return null
            val nid = data["nid"].nonEmpty() ?: return null
            val kind = data["k"].nonEmpty() ?: return null
            val instance = data["i"].nonEmpty() ?: return null
            val badge = data["badge"]?.toIntOrNull()?.takeIf { it >= 0 }
            if (kind == SYNC) {
                val cleared = data["cleared"].orEmpty().split(',').map(String::trim).filter(String::isNotEmpty)
                return PushSync(nid = nid, instanceId = instance, badge = badge, cleared = cleared)
            }
            return PushHint(
                nid = nid,
                kind = kind,
                instanceId = instance,
                threadId = data["t"].nonEmpty(),
                messageId = data["m"].nonEmpty(),
                badge = badge,
                quiet = data["quiet"] == "1",
            )
        }

        private fun String?.nonEmpty(): String? = this?.takeIf { it.isNotEmpty() }
    }
}

/**
 * One notification of a kind of 13 2.
 *
 * @property kind `k`, the wire name of the kind.
 * @property threadId `t`, absent for `security` and `admin`.
 * @property messageId `m`, absent without a message.
 * @property quiet `quiet = "1"`: posted without sound or vibration (13 3.4).
 */
data class PushHint(
    override val nid: String,
    val kind: String,
    override val instanceId: String,
    val threadId: String?,
    val messageId: String?,
    override val badge: Int?,
    val quiet: Boolean,
) : PushPayload {
    /** Whether the kind points at a message, so a preview exists (13 6.4 check 1). */
    val hasPreview: Boolean
        get() = kind in PREVIEW_KINDS && threadId != null && messageId != null

    /**
     * The tag of the notification: the thread, so a later push of the thread replaces it (13
     * 5.2); a hint without a thread replaces the earlier one of its kind.
     */
    val tag: String
        get() = threadId ?: "kind:$kind"

    // IDs only, but no log line needs them.
    override fun toString(): String = "PushHint(kind=$kind, quiet=$quiet)"

    companion object {
        /** The kinds that point at a message (13 6.4 check 1). */
        val PREVIEW_KINDS = setOf("message", "mention", "dm", "run_done", "run_failed", "input_needed")
    }
}

/**
 * The sync form of 13 8: the badge number and the threads read on another device.
 *
 * @property cleared the thread IDs of `cleared`, in their order; empty when absent.
 */
data class PushSync(
    override val nid: String,
    override val instanceId: String,
    override val badge: Int?,
    val cleared: List<String>,
) : PushPayload {
    override fun toString(): String = "PushSync(cleared=${cleared.size})"
}
