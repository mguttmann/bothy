// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import com.example.client.MainActivity
import com.example.client.R
import java.util.concurrent.ConcurrentHashMap

/**
 * One lock per notification tag for the whole process: the service records the newest `nid` and
 * posts its placeholder under it, and the worker checks that its `nid` is still the newest and
 * posts the text under it, so an older fetch never replaces a newer notification (brief M5-05
 * Design note 2).
 */
object TagLocks {
    private val locks = ConcurrentHashMap<String, Any>()

    /** Runs [body] under the lock of [tag]. */
    fun <T> withLock(tag: String, body: () -> T): T = synchronized(locks.computeIfAbsent(tag) { Any() }) { body() }
}

/**
 * Posts, replaces and clears the push notifications (docs/spec/13-push-notifications.md 5.2, 8;
 * 15 10.2; brief M5-05 Design notes 1 and 3).
 *
 * Every notification goes into the channel of its kind (the mapping of M5-03), carries the tag
 * of its thread with the group of the same thread, so a later push of the thread replaces it,
 * and opens the thread at the message when tapped. A quiet hint (`quiet = 1`, 13 3.4) posts
 * without sound or vibration: its group alerts only through a summary, and none is posted.
 *
 * The system sheds updates of an app's notifications above its enqueue rate (a few per second),
 * so in a burst of pushes a replacement can stay unshown and the placeholder stays; nothing
 * tells the app (D-M5-05-2). [posted] sees every notification handed to the system, which the
 * instrumented tests read instead of the drawer for updates.
 *
 * @param context the application context.
 * @param texts the localized texts.
 * @param channelOf the channel ID of a kind.
 * @param posted sees the tag and the notification of every `notify`; null in production.
 */
class NotificationPoster(
    private val context: Context,
    private val texts: PushTexts,
    private val channelOf: (String) -> String,
    private val posted: ((String, Notification) -> Unit)? = null,
) {
    private val manager = context.getSystemService(NotificationManager::class.java)

    private fun notify(tag: String, notification: Notification) {
        posted?.invoke(tag, notification)
        manager.notify(tag, ID, notification)
    }

    /** Posts the placeholder of [hint] at once: the title and the body of its kind. */
    fun postPlaceholder(hint: PushHint, receivedAtMillis: Long) {
        notify(hint.tag, build(hint, texts.title, texts.bodyOf(hint.kind), hint.badge, receivedAtMillis, alertOnce = false))
    }

    /** Replaces the notification of [hint] with [title] and [body]; it does not alert again. */
    fun postText(hint: PushHint, title: String, body: String, badge: Int?, receivedAtMillis: Long) {
        notify(hint.tag, build(hint, title, body, badge ?: hint.badge, receivedAtMillis, alertOnce = true))
    }

    /** Replaces the notification of [hint] with "Message not available" (`404`, 13 6.4). */
    fun postNotAvailable(hint: PushHint, receivedAtMillis: Long) {
        notify(hint.tag, build(hint, texts.title, texts.notAvailable, hint.badge, receivedAtMillis, alertOnce = true))
    }

    /**
     * The sync of 13 8: removes the notifications of the cleared threads and sets the badge
     * number on the ones that stay (`setNumber`, for launchers that show it); posts nothing new.
     *
     * @return the tags removed.
     */
    fun sync(sync: PushSync, store: PushStore): List<String> {
        val removed = mutableListOf<String>()
        for (tag in sync.cleared) {
            TagLocks.withLock(tag) {
                manager.cancel(tag, ID)
                store.clearNewest(tag)
            }
            removed += tag
        }
        val badge = sync.badge ?: return removed
        for (shown in manager.activeNotifications) {
            val tag = shown.tag ?: continue
            if (shown.id != ID || tag in sync.cleared) continue
            TagLocks.withLock(tag) {
                val updated =
                    Notification.Builder.recoverBuilder(context, shown.notification)
                        .setNumber(badge)
                        .setOnlyAlertOnce(true)
                        .build()
                notify(tag, updated)
            }
        }
        return removed
    }

    /** Removes every push notification (the account ended). */
    fun clearAll() {
        for (shown in manager.activeNotifications) if (shown.id == ID) manager.cancel(shown.tag, ID)
    }

    private fun build(
        hint: PushHint,
        title: String,
        body: String,
        badge: Int?,
        receivedAtMillis: Long,
        alertOnce: Boolean,
    ): Notification {
        val builder =
            Notification.Builder(context, channelOf(hint.kind))
                .setSmallIcon(R.drawable.ic_notification)
                .setContentTitle(title)
                .setContentText(body)
                .setStyle(Notification.BigTextStyle().bigText(body))
                .setCategory(Notification.CATEGORY_MESSAGE)
                .setWhen(receivedAtMillis)
                .setShowWhen(true)
                .setAutoCancel(true)
                .setGroup(hint.tag)
                .setOnlyAlertOnce(alertOnce)
                .setContentIntent(tapIntent(hint))
        if (badge != null) builder.setNumber(badge)
        // No summary of the group is ever posted, so the children never alert.
        if (hint.quiet) builder.setGroupAlertBehavior(Notification.GROUP_ALERT_SUMMARY)
        return builder.build()
    }

    private fun tapIntent(hint: PushHint): PendingIntent {
        val intent =
            Intent(context, MainActivity::class.java)
                .setAction(ACTION_TAP)
                .putExtra(EXTRA_INSTANCE, hint.instanceId)
                .putExtra(EXTRA_THREAD, hint.threadId)
                .putExtra(EXTRA_MESSAGE, hint.messageId)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(
            context,
            hint.tag.hashCode(),
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
    }

    /** The notification ID and the tap intent's names. */
    companion object {
        /** The ID of every push notification; the tag tells them apart. */
        const val ID = 0x5055

        /** The action of the intent a tapped notification sends to the main activity. */
        const val ACTION_TAP = "com.example.client.push.TAP"

        /** `i` of the payload. */
        const val EXTRA_INSTANCE = "push.instance"

        /** `t` of the payload. */
        const val EXTRA_THREAD = "push.thread"

        /** `m` of the payload. */
        const val EXTRA_MESSAGE = "push.message"
    }
}
