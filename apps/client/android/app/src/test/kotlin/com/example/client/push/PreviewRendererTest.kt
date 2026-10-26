// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Test

/** The content rules of the NSE (D-M5-04-4) on Android. */
class PreviewRendererTest {
    private val texts =
        PushTexts(
            title = "New notification",
            bodies = mapOf("message" to "New message", "mention" to "You were mentioned"),
            notAvailable = "Message not available",
            titleFormat = "%1\$s · %2\$s",
            textFormat = "%1\$s: %2\$s",
        )

    private fun preview(channel: String? = null, thread: String? = null, sender: String? = null, excerpt: String? = null) =
        PushPreview("mention", 1, channel, thread, sender, excerpt)

    @Test
    fun fullShowsChannelThreadSenderAndExcerpt() {
        assertEquals(
            "tickets · Invoice export fails" to "Alex: Sam can you check 100%s?",
            PreviewRenderer.render(preview("tickets", "Invoice export fails", "Alex", "Sam can you check 100%s?"), "mention", texts),
        )
    }

    @Test
    fun namesShowsThePlaceholderBodyAfterTheSender() {
        assertEquals("tickets" to "Alex: You were mentioned", PreviewRenderer.render(preview("tickets", null, "Alex"), "mention", texts))
    }

    @Test
    fun noneKeepsThePlaceholder() {
        assertEquals("New notification" to "New message", PreviewRenderer.render(preview(), "later_kind", texts))
    }

    @Test
    fun aBrokenFormatStillShowsBothParts() {
        assertEquals("a b", PreviewRenderer.format("%1\$s only", "a", "b"))
    }
}
