// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import com.example.client.NotificationChannelRegistrar
import org.junit.Assert.assertEquals
import org.junit.Test

/** The kind to channel mapping of M5-03 (D-M5-03-4) as the push service resolves it. */
class KindToChannelTest {
    // The channels as the app hands them over (PushChannel of lib/platform/notification_channels.dart).
    private val mapping =
        NotificationChannelRegistrar.kindsOf(
            listOf(
                "mentions" to listOf("mention", "dm"),
                "questions" to listOf("input_needed", "security", "admin"),
                "messages" to listOf("message"),
                "runs" to listOf("run_done", "run_failed"),
            ),
        )

    private fun channelOf(kind: String) = NotificationChannelRegistrar.resolve(kind) { mapping[it] }

    @Test
    fun everyKindGoesToItsChannel() {
        val expected =
            mapOf(
                "mention" to "mentions",
                "dm" to "mentions",
                "input_needed" to "questions",
                "security" to "questions",
                "admin" to "questions",
                "message" to "messages",
                "run_done" to "runs",
                "run_failed" to "runs",
            )
        for ((kind, channel) in expected) assertEquals(kind, channel, channelOf(kind))
    }

    @Test
    fun anUnknownKindGoesToMessages() {
        assertEquals("messages", channelOf("later_kind"))
        assertEquals("messages", NotificationChannelRegistrar.resolve("mention") { null })
    }
}
