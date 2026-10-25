// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The FCM `data` of 13 6.2 and the sync form of 13 8 are read as the server sends them. */
class PayloadParsingTest {
    private fun data(kind: String, vararg extra: Pair<String, String>) =
        mapOf(
            "v" to "1",
            "nid" to "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d",
            "k" to kind,
            "i" to "0192a000-0000-7000-8000-00000000c0de",
            "t" to "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11",
            "m" to "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8",
            "badge" to "3",
            "quiet" to "0",
        ) + extra

    @Test
    fun everyKindOfSection2IsAHint() {
        for (kind in listOf("message", "mention", "dm", "run_done", "run_failed", "input_needed")) {
            val hint = PushPayload.parse(data(kind)) as PushHint
            assertEquals(kind, hint.kind)
            assertEquals("0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", hint.threadId)
            assertEquals("0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8", hint.messageId)
            assertEquals(3, hint.badge)
            assertFalse(hint.quiet)
            assertTrue(kind, hint.hasPreview)
            assertEquals(hint.threadId, hint.tag)
        }
        for (kind in listOf("security", "admin")) {
            val hint = PushPayload.parse(data(kind) - "t" - "m") as PushHint
            assertFalse(kind, hint.hasPreview)
            assertEquals("kind:$kind", hint.tag)
        }
    }

    @Test
    fun quietIsOnlyTheValueOne() {
        assertTrue((PushPayload.parse(data("message", "quiet" to "1")) as PushHint).quiet)
        assertFalse((PushPayload.parse(data("message", "quiet" to "true")) as PushHint).quiet)
    }

    @Test
    fun anUnknownVersionOrAMissingKeyIsIgnored() {
        assertNull(PushPayload.parse(data("message", "v" to "2")))
        assertNull(PushPayload.parse(data("message") - "v"))
        assertNull(PushPayload.parse(data("message") - "nid"))
        assertNull(PushPayload.parse(data("message", "i" to "")))
        assertNull(PushPayload.parse(data("message") - "k"))
    }

    @Test
    fun anUnknownKindStaysAHintWithoutPreview() {
        val hint = PushPayload.parse(data("later_kind")) as PushHint
        assertFalse(hint.hasPreview)
    }

    @Test
    fun syncCarriesTheBadgeAndThreeClearedThreads() {
        val sync =
            PushPayload.parse(
                mapOf(
                    "v" to "1",
                    "nid" to "n-1",
                    "k" to "sync",
                    "i" to "inst",
                    "badge" to "0",
                    "cleared" to "t-1,t-2,t-3",
                ),
            ) as PushSync
        assertEquals(listOf("t-1", "t-2", "t-3"), sync.cleared)
        assertEquals(0, sync.badge)
        val bare = PushPayload.parse(mapOf("v" to "1", "nid" to "n-2", "k" to "sync", "i" to "inst", "badge" to "2")) as PushSync
        assertEquals(emptyList<String>(), bare.cleared)
    }

    @Test
    fun aLogLineCarriesNoId() {
        val hint = PushPayload.parse(data("dm")) as PushHint
        assertFalse(hint.toString().contains("0192"))
    }
}
