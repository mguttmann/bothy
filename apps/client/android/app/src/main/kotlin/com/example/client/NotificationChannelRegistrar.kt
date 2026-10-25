// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context

/**
 * The notification channels of 01 10.3 (15 10.2; brief M5-03 Design note 4;
 * docs/decisions.d/M5-03.md D-M5-03-4).
 *
 * The app hands the channels at every start with names and descriptions in the device
 * language. `createNotificationChannel` creates a missing channel and updates the name and the
 * description of an existing one; the importance stays what the person chose in the system
 * settings. The kind to channel mapping is kept for the push service of M5-05, which posts
 * without a running Flutter engine.
 */
class NotificationChannelRegistrar(private val context: Context) {
    /** Creates or updates every channel of [channels] and keeps their kinds. */
    fun register(channels: List<AndroidNotificationChannel>) {
        val manager = context.getSystemService(NotificationManager::class.java)
        for (channel in channels) {
            val importance =
                if (channel.high) NotificationManager.IMPORTANCE_HIGH else NotificationManager.IMPORTANCE_DEFAULT
            manager.createNotificationChannel(
                NotificationChannel(channel.id, channel.name, importance).apply {
                    description = channel.description
                },
            )
        }
        val mapping = context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE).edit().clear()
        for ((kind, id) in kindsOf(channels.map { it.id to it.kinds })) mapping.putString(kind, id)
        mapping.apply()
    }

    /** The channel ID for the notification kind [kind]; an unknown kind goes to `messages`. */
    fun channelOf(kind: String): String =
        resolve(kind) { context.getSharedPreferences(PREFERENCES, Context.MODE_PRIVATE).getString(it, null) }

    /** The mapping rules, without Android, so the JVM tests hold them. */
    companion object {
        private const val PREFERENCES = "notification_channels"

        /** The channel of a kind the mapping does not know (a newer server). */
        const val FALLBACK = "messages"

        /**
         * The kind to channel ID mapping of [channels] (channel ID and its kinds, as the app
         * hands them over).
         */
        fun kindsOf(channels: List<Pair<String, List<String>>>): Map<String, String> =
            channels.flatMap { (id, kinds) -> kinds.map { it to id } }.toMap()

        /** The channel of [kind] by [stored] (the kept mapping), or [FALLBACK]. */
        fun resolve(kind: String, stored: (String) -> String?): String = stored(kind) ?: FALLBACK
    }
}
