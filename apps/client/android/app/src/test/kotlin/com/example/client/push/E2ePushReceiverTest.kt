// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Base64

/**
 * The debug receiver of the device E2E (brief M5-11 Design note 2) decodes `data_b64` as the
 * runner's relay writes it (standard base64 of a JSON object of strings) and hands the map to
 * the push receiver unchanged; anything else is dropped without a hand-over.
 *
 * `android.util.Log` is not mocked in these tests, so a log call on any path below would fail
 * them: decoding and hand-over log nothing, in particular no content.
 */
class E2ePushReceiverTest {
    private fun b64(json: String): String = Base64.getEncoder().encodeToString(json.toByteArray(Charsets.UTF_8))

    private fun b64(bytes: ByteArray): String = Base64.getEncoder().encodeToString(bytes)

    // The `data` of an FCM message of 13 6.2 as the fake provider captures it.
    private val hint =
        mapOf(
            "v" to "1",
            "nid" to "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d",
            "k" to "message",
            "i" to "0192a000-0000-7000-8000-00000000c0de",
            "t" to "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11",
            "m" to "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8",
            "badge" to "2",
            "quiet" to "0",
        )

    @Test
    fun theDataOfAMessageReachesThePushReceiverUnchanged() {
        val json =
            """{"v": "1", "nid": "0192f1d3-0a55-7b21-8c4d-5e6f7a8b9c0d", "k": "message", """ +
                """"i": "0192a000-0000-7000-8000-00000000c0de", "t": "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", """ +
                """"m": "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8", "badge": "2", "quiet": "0"}"""
        val handed = mutableListOf<Map<String, String>>()
        assertTrue(E2ePushReceiver.deliver(b64(json)) { handed += it })
        assertEquals(listOf(hint), handed)
        // The push receiver reads it as the FCM service's message.
        val parsed = PushPayload.parse(handed.single()) as PushHint
        assertEquals("message", parsed.kind)
        assertEquals("0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", parsed.threadId)
        assertTrue(parsed.hasPreview)
    }

    @Test
    fun escapesAndUnicodeDecodeAsJson() {
        // Python's json.dumps escapes every non-ASCII character; raw UTF-8 is valid JSON too.
        val json =
            "{ \"a\" : \"q\\\"b\\\\s\\/n\\n\\t\\r\\b\\f\" ,\n\t\"\\u00e9\": \"\\ud83d\\ude00\", " +
                "\"raw\":\"\u00e9\ud83d\ude00\", \"empty\": \"\" }"
        assertEquals(
            mapOf(
                "a" to "q\"b\\s/n\n\t\r\b\u000c",
                "\u00e9" to "\ud83d\ude00",
                "raw" to "\u00e9\ud83d\ude00",
                "empty" to "",
            ),
            E2ePushData.decode(b64(json)),
        )
    }

    @Test
    fun anEmptyObjectIsHandedOverAndThePushReceiverIgnoresIt() {
        val handed = mutableListOf<Map<String, String>>()
        assertTrue(E2ePushReceiver.deliver(b64(" {} ")) { handed += it })
        assertEquals(listOf(emptyMap<String, String>()), handed)
        assertNull(PushPayload.parse(handed.single()))
    }

    @Test
    fun malformedInputIsDroppedWithoutAHandOver() {
        val malformed =
            listOf(
                null,
                "",
                "not base64!",
                // Base64 with a line break, as `base64` without `-w0` writes long input.
                b64("{\"a\":\"b\"}").let { it.substring(0, 4) + "\n" + it.substring(4) },
                b64(""),
                b64("[]"),
                b64("\"a\""),
                b64("{\"a\":1}"),
                b64("{\"a\":null}"),
                b64("{\"a\":true}"),
                b64("{\"a\":{\"b\":\"c\"}}"),
                b64("{\"a\":[\"b\"]}"),
                b64("{\"a\":\"b\"} x"),
                b64("{\"a\":\"b\"}{}"),
                b64("{\"a\":\"b\",}"),
                b64("{,}"),
                b64("{\"a\" \"b\"}"),
                b64("{\"a\":\"b\" \"c\":\"d\"}"),
                b64("{\"a\":\"b"),
                b64("{\"a\":\"b\""),
                b64("{\"a\":\"\\x\"}"),
                b64("{\"a\":\"\\u12\"}"),
                b64("{\"a\":\"\\u+123\"}"),
                b64("{\"a\":\"\\u12g4\"}"),
                b64("{\"a\":\"b\\"),
                b64("{\"a\":\"tab\there\"}"),
                b64("{'a':'b'}"),
                b64("{a:\"b\"}"),
                b64("{\"a\":\"b\",\"a\":\"c\"}"),
                // `{"a":"` followed by an invalid UTF-8 sequence and `"}`.
                b64(byteArrayOf(0x7b, 0x22, 0x61, 0x22, 0x3a, 0x22, 0xc3.toByte(), 0x28, 0x22, 0x7d)),
            )
        for (input in malformed) {
            var handedOver = false
            assertFalse(input.toString(), E2ePushReceiver.deliver(input) { handedOver = true })
            assertFalse(input.toString(), handedOver)
            assertNull(input.toString(), E2ePushData.decode(input))
        }
    }

    @Test
    fun theActionBelongsToTheApplicationId() {
        // The relay sends `<application ID>.E2E_PUSH` with the extra `data_b64` (contract of M5-11).
        assertEquals("com.example.chat.E2E_PUSH", E2ePushReceiver.action("com.example.chat"))
        assertEquals("data_b64", E2ePushReceiver.EXTRA_DATA)
    }
}
