// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

/**
 * What the push service does with one FCM data message, without Firebase types, so the build
 * without Firebase compiles it and the tests drive it (docs/spec/13-push-notifications.md 5.2,
 * 6.2, 8; brief M5-05 Design notes 1 and 3; D-M5-05-1).
 *
 * A hint posts its placeholder at once (no network here, the service has only the FCM budget),
 * records its `nid` as the newest of its tag under the tag's lock and, when a preview can be
 * fetched, schedules the [PreviewWorker] of this `nid`. With `notify.preview_mode = none`, no
 * stored preview token for the instance or no written fetch timeout nothing is fetched and the
 * placeholder stays. A sync removes the cleared threads and sets the badge; it never fetches.
 *
 * @param runtime the process's push runtime.
 */
class PushReceiver(private val runtime: PushRuntime) {
    /**
     * Handles the `data` of one message.
     *
     * @return what was done, for the tests.
     */
    fun onMessage(data: Map<String, String>): Handled {
        return when (val payload = PushPayload.parse(data)) {
            null -> Handled.IGNORED
            is PushSync -> {
                runtime.poster().sync(payload, runtime.store)
                Handled.SYNCED
            }
            is PushHint -> onHint(payload)
        }
    }

    private fun onHint(hint: PushHint): Handled {
        val store = runtime.store
        val receivedAt = runtime.clock()
        TagLocks.withLock(hint.tag) {
            store.setNewest(hint.tag, hint.nid)
            runtime.poster().postPlaceholder(hint, receivedAt)
        }
        if (!hint.hasPreview) return Handled.POSTED
        val mode = store.previewMode()
        val fetch =
            mode != null && mode != NONE && store.fetchTimeoutMillis() != null &&
                store.registration(hint.instanceId) != null
        if (!fetch) {
            store.record((runtime.clock() - receivedAt) / 1000.0, placeholder = true)
            return Handled.POSTED
        }
        runtime.scheduler.schedule(PreviewRequest(hint, receivedAt))
        return Handled.SCHEDULED
    }

    /** What [onMessage] did. */
    enum class Handled {
        /** Not a message of payload version 1. */
        IGNORED,

        /** The placeholder is the final notification. */
        POSTED,

        /** The placeholder is posted and the preview fetch scheduled. */
        SCHEDULED,

        /** A sync was applied. */
        SYNCED,
    }

    private companion object {
        const val NONE = "none"
    }
}
