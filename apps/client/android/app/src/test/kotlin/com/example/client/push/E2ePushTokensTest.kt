// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Base64
import java.util.Random

/**
 * The fake FCM token of the device E2E build (brief M5-11 Design note 2) is random, made once
 * per install and kept in the app's private storage, so every start registers the same token.
 */
class E2ePushTokensTest {
    private class MemoryValues : KeyValueStore {
        val map = linkedMapOf<String, String>()

        override fun get(key: String) = map[key]

        override fun put(key: String, value: String?) {
            if (value == null) map.remove(key) else map[key] = value
        }

        override fun keys() = map.keys.toSet()
    }

    private val values = MemoryValues()

    @Test
    fun theFirstCallMakesARandomTokenThatRegisterPushAccepts() {
        val token = E2ePushTokens(values, Random(1)).token()
        val bytes = ByteArray(32).also { Random(1).nextBytes(it) }
        assertEquals(E2ePushTokens.PREFIX + Base64.getUrlEncoder().withoutPadding().encodeToString(bytes), token)
        // An `fcm` token of registerPush is printable ASCII without white space, and fits the
        // smallest allowed `push.registration_field_max_bytes` (docs/spec/13-push-notifications.md).
        assertTrue(token, token.all { it in '!'..'~' })
        assertTrue(token, token.length <= 64)
        assertEquals(mapOf(E2ePushTokens.KEY to token), values.map)
    }

    @Test
    fun theTokenStaysTheSameForTheInstall() {
        val first = E2ePushTokens(values, Random(1)).token()
        assertEquals(first, E2ePushTokens(values, Random(1)).token())
        // A later start (a new store object over the same file) reads the stored token.
        assertEquals(first, E2ePushTokens(values, Random(2)).token())
    }

    @Test
    fun anotherInstallGetsAnotherToken() {
        assertNotEquals(
            E2ePushTokens(MemoryValues(), Random(1)).token(),
            E2ePushTokens(MemoryValues(), Random(2)).token(),
        )
    }
}
