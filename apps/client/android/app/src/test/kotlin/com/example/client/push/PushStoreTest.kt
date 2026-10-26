// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.spec.GCMParameterSpec

/** The push store keeps no content, seals every value and round-trips with an in-memory key. */
class PushStoreTest {
    private class MemoryValues : KeyValueStore {
        val map = linkedMapOf<String, String>()

        override fun get(key: String) = map[key]

        override fun put(key: String, value: String?) {
            if (value == null) map.remove(key) else map[key] = value
        }

        override fun keys() = map.keys.toSet()
    }

    // AES-GCM with a key in memory, the same construction as KeystoreCipher.
    private class MemoryCipher : ValueCipher {
        private val key = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()

        override fun encrypt(plain: ByteArray, context: ByteArray): ByteArray {
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.ENCRYPT_MODE, key)
            cipher.updateAAD(context)
            return cipher.iv + cipher.doFinal(plain)
        }

        override fun decrypt(sealed: ByteArray, context: ByteArray): ByteArray {
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, sealed, 0, 12))
            cipher.updateAAD(context)
            return cipher.doFinal(sealed, 12, sealed.size - 12)
        }
    }

    private val values = MemoryValues()
    private val store = PushStore(values, MemoryCipher())
    private val instance = "0192a000-0000-7000-8000-00000000c0de"
    private val token = "bothy_pvw_EXAMPLEpushpreviewtoken00000000000000000000_4Z1L0A"

    @Test
    fun aRegistrationRoundTripsAndNothingIsReadableInTheFile() {
        store.saveRegistration(instance, PushRegistration("https://chat.example.com", "reg-1", token))
        store.writeSettings(instance, "full", 20_000)
        store.rememberNonce("https://chat.example.com", "nonce-1")
        store.setNewest("0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", "nid-1")
        val stored = store.registration(instance)!!
        assertEquals("https://chat.example.com", stored.serverUrl)
        assertEquals("reg-1", stored.registrationId)
        assertEquals(token, stored.previewToken)
        assertEquals("full", store.previewMode())
        assertEquals(20_000L, store.fetchTimeoutMillis())
        assertEquals("nonce-1", store.nonce("https://chat.example.com"))
        assertEquals("nid-1", store.newest("0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11"))
        val file = values.map.entries.joinToString("\n") { "${it.key}=${it.value}" }
        for (secret in listOf(token, "chat.example.com", instance, "0192f1d2", "reg-1", "nonce-1", "nid-1")) {
            assertFalse(secret, file.contains(secret))
        }
        assertFalse(stored.toString().contains(token))
    }

    @Test
    fun theKeysNameNoContent() {
        store.saveRegistration(instance, PushRegistration("https://chat.example.com", "reg-1", token))
        store.writeSettings(instance, "names", 1_000)
        store.record(0.3, placeholder = true)
        val allowed = Regex("(reg\\.[0-9a-f]{64}\\.(server_url|registration_id|preview_token)|signed_in_instance|preview_mode|fetch_timeout_ms|nonce\\.[0-9a-f]{64}|newest\\.[0-9a-f]{64}|counters\\.(buckets|placeholders))")
        for (key in values.keys()) assertEquals(key, true, allowed.matches(key))
        for (word in listOf("title", "excerpt", "sender", "channel", "name", "text", "body")) {
            assertFalse(word, values.keys().any { it.contains(word) && !it.startsWith("reg.") })
        }
    }

    @Test
    fun aChangedValueReadsAsAbsent() {
        store.writeSettings(instance, "full", 20_000)
        values.map["preview_mode"] = values.map.getValue("fetch_timeout_ms")
        assertNull(store.previewMode())
    }

    @Test
    fun deletingTheSignedInInstanceClearsItsSettings() {
        store.saveRegistration(instance, PushRegistration("https://chat.example.com", "reg-1", token))
        store.writeSettings(instance, "full", 20_000)
        store.setNewest("t", "n")
        store.deleteRegistration(instance)
        assertNull(store.registration(instance))
        assertNull(store.previewMode())
        assertNull(store.newest("t"))
    }

    @Test
    fun countersFallIntoTheBucketsAndReset() {
        store.record(0.004, placeholder = false)
        store.record(0.3, placeholder = true)
        store.record(99.0, placeholder = true)
        val counts = store.takeCounts()
        assertEquals(DisplayBuckets.BOUNDS.size, counts.buckets.size)
        assertEquals(1, counts.buckets[0])
        assertEquals(1, counts.buckets[DisplayBuckets.BOUNDS.indexOf(0.5)])
        assertEquals(1, counts.buckets.last())
        assertEquals(2, counts.placeholders)
        assertEquals(DisplayCounts(List(DisplayBuckets.BOUNDS.size) { 0 }, 0), store.takeCounts())
    }
}
