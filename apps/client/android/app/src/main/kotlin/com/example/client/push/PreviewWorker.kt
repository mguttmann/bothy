// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.app.Notification
import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.ExistingWorkPolicy
import androidx.work.ForegroundInfo
import androidx.work.OneTimeWorkRequest
import androidx.work.OutOfQuotaPolicy
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.example.client.NotificationChannelRegistrar
import com.example.client.R
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * The preview fetch of one push: the payload's IDs and the time of receipt.
 *
 * @property hint the push.
 * @property receivedAtMillis when `onMessageReceived` posted the placeholder, Unix milliseconds.
 */
data class PreviewRequest(val hint: PushHint, val receivedAtMillis: Long) {
    /** The worker input; IDs only, never a token. */
    fun toData(): Data =
        Data.Builder()
            .putString(NID, hint.nid)
            .putString(KIND, hint.kind)
            .putString(INSTANCE, hint.instanceId)
            .putString(THREAD, hint.threadId)
            .putString(MESSAGE, hint.messageId)
            .putInt(BADGE, hint.badge ?: -1)
            .putBoolean(QUIET, hint.quiet)
            .putLong(RECEIVED, receivedAtMillis)
            .build()

    /** The input keys. */
    companion object {
        private const val NID = "nid"
        private const val KIND = "k"
        private const val INSTANCE = "i"
        private const val THREAD = "t"
        private const val MESSAGE = "m"
        private const val BADGE = "badge"
        private const val QUIET = "quiet"
        private const val RECEIVED = "received_at"

        /** Reads the worker input [data]; null when a key is missing. */
        fun of(data: Data): PreviewRequest? {
            val hint =
                PushHint(
                    nid = data.getString(NID) ?: return null,
                    kind = data.getString(KIND) ?: return null,
                    instanceId = data.getString(INSTANCE) ?: return null,
                    threadId = data.getString(THREAD),
                    messageId = data.getString(MESSAGE),
                    badge = data.getInt(BADGE, -1).takeIf { it >= 0 },
                    quiet = data.getBoolean(QUIET, false),
                )
            val received = data.getLong(RECEIVED, -1).takeIf { it >= 0 } ?: return null
            return PreviewRequest(hint, received)
        }
    }
}

/** Starts the preview fetch of a push. */
fun interface PreviewScheduler {
    /** Schedules [request]. */
    fun schedule(request: PreviewRequest)
}

/**
 * [PreviewScheduler] over WorkManager: one expedited [PreviewWorker] per `nid` (unique work, so
 * a second delivery of the same push starts nothing), run as normal work when the expedited
 * quota is used up; that late run then keeps the placeholder (D-M5-05-1, D-M5-05-2).
 */
class WorkManagerScheduler(private val context: Context) : PreviewScheduler {
    override fun schedule(request: PreviewRequest) {
        WorkManager.getInstance(context)
            .enqueueUniqueWork(uniqueName(request.hint.nid), ExistingWorkPolicy.KEEP, workRequest(request))
    }

    /** The work request. */
    companion object {
        /** The unique work name of the push [nid]. */
        fun uniqueName(nid: String): String = "push-preview-$nid"

        /** The expedited request of [request]. */
        fun workRequest(request: PreviewRequest): OneTimeWorkRequest =
            OneTimeWorkRequest.Builder(PreviewWorker::class.java)
                .setExpedited(OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST)
                .setInputData(request.toData())
                .build()
    }
}

/** How a preview fetch ended. */
enum class PreviewOutcome {
    /** The text replaced the placeholder. */
    TEXT,

    /** `404`: "Message not available" replaced the placeholder. */
    NOT_AVAILABLE,

    /** The placeholder stayed: no network, an error, `none` or no credentials. */
    PLACEHOLDER,

    /** The run started or ended after `client.push.fetch_timeout`; the placeholder stayed. */
    LATE,

    /** A newer push of the same tag arrived; this one posted nothing. */
    STALE,
}

/**
 * The work of one [PreviewWorker] run (brief M5-05 Design note 2; D-M5-05-2).
 *
 * It fetches the preview within what is left of `client.push.fetch_timeout` since receipt, then,
 * under the lock of the tag, posts the text (or "Message not available" on `404`) only when its
 * `nid` is still the newest of the tag and the time since receipt is within the timeout. A
 * failure keeps the placeholder and is never retried: a late text would replace a notification
 * the person may have read. It counts the duration from receipt and whether the placeholder
 * stayed.
 *
 * @param runtime the process's push runtime.
 */
class PreviewJob(private val runtime: PushRuntime) {
    /** Runs the fetch of [request]. */
    fun run(request: PreviewRequest): PreviewOutcome {
        val hint = request.hint
        val store = runtime.store
        val timeout = store.fetchTimeoutMillis()
        val registration = store.registration(hint.instanceId)
        val mode = store.previewMode()
        val messageId = hint.messageId
        val left = timeout?.let { it - (runtime.clock() - request.receivedAtMillis) }
        val result =
            when {
                timeout == null || left == null || left <= 0 -> null
                registration == null || messageId == null || mode == null || mode == NONE -> PreviewResult.Failed
                else -> {
                    val signer = runtime.signer(hint.instanceId)
                    if (signer == null) {
                        PreviewResult.Failed
                    } else {
                        PreviewClient(registration, signer, store, runtime.connector) { runtime.clock() / 1000 }
                            .fetch(hint.nid, messageId, hint.kind, left.coerceAtMost(Int.MAX_VALUE.toLong()).toInt())
                    }
                }
            }
        return TagLocks.withLock(hint.tag) { finish(request, timeout, result) }
    }

    private fun finish(request: PreviewRequest, timeout: Long?, result: PreviewResult?): PreviewOutcome {
        val hint = request.hint
        val store = runtime.store
        if (store.newest(hint.tag) != hint.nid) return PreviewOutcome.STALE
        val elapsed = runtime.clock() - request.receivedAtMillis
        val seconds = elapsed / 1000.0
        if (result == null || timeout == null || elapsed > timeout) {
            store.record(seconds, placeholder = true)
            return PreviewOutcome.LATE
        }
        val poster = runtime.poster()
        return when (result) {
            is PreviewResult.Content -> {
                val (title, body) = PreviewRenderer.render(result.preview, hint.kind, PushTexts.of(runtime.context))
                poster.postText(hint, title, body, result.preview.badge, request.receivedAtMillis)
                store.record(seconds, placeholder = false)
                PreviewOutcome.TEXT
            }
            PreviewResult.NotAvailable -> {
                poster.postNotAvailable(hint, request.receivedAtMillis)
                store.record(seconds, placeholder = false)
                PreviewOutcome.NOT_AVAILABLE
            }
            PreviewResult.Failed -> {
                store.record(seconds, placeholder = true)
                PreviewOutcome.PLACEHOLDER
            }
        }
    }

    private companion object {
        const val NONE = "none"
    }
}

/**
 * The expedited worker of the preview fetch: Kotlin only, no Flutter engine (15 10.2; ADR 0056
 * item 3). It always ends with success, so WorkManager never retries.
 */
class PreviewWorker(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        val request = PreviewRequest.of(inputData) ?: return Result.success()
        withContext(Dispatchers.IO) { PreviewJob(PushRuntime.of(applicationContext)).run(request) }
        return Result.success()
    }

    /**
     * Below API 31 expedited work runs as a foreground service, which needs a notification: a
     * silent one in the channel of the push's kind with the placeholder title, gone when the
     * fetch ends.
     */
    override suspend fun getForegroundInfo(): ForegroundInfo {
        val kind = PreviewRequest.of(inputData)?.hint?.kind ?: "message"
        val notification =
            Notification.Builder(applicationContext, NotificationChannelRegistrar(applicationContext).channelOf(kind))
                .setSmallIcon(R.drawable.ic_notification)
                .setContentTitle(applicationContext.getString(R.string.push_title))
                .setGroup(FOREGROUND_GROUP)
                .setGroupAlertBehavior(Notification.GROUP_ALERT_SUMMARY)
                .build()
        return ForegroundInfo(FOREGROUND_ID, notification)
    }

    private companion object {
        const val FOREGROUND_ID = 0x5056
        const val FOREGROUND_GROUP = "push.fetch"
    }
}
