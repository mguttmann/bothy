// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import java.security.MessageDigest
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Plain string values by key; [SharedPreferencesValues] on the device, a map in the JVM tests. */
interface KeyValueStore {
    /** The value of [key], or null. */
    fun get(key: String): String?

    /** Writes [value] under [key], or removes [key] when [value] is null; durable on return. */
    fun put(key: String, value: String?)

    /** Every key present. */
    fun keys(): Set<String>
}

/**
 * Encrypts and decrypts one value; [KeystoreCipher] on the device, an in-memory key in the JVM
 * tests.
 */
interface ValueCipher {
    /** Encrypts [plain] bound to [context] (authenticated, not encrypted). */
    fun encrypt(plain: ByteArray, context: ByteArray): ByteArray

    /**
     * Decrypts [sealed] that [encrypt] made with the same [context].
     *
     * @throws Exception when the value was changed, belongs to another key or [context].
     */
    fun decrypt(sealed: ByteArray, context: ByteArray): ByteArray
}

/**
 * AES-GCM with a key that never leaves the Android Keystore (brief M5-05 Design note 4;
 * docs/decisions.d/M5-05.md D-M5-05-4): its own alias, not one of the device keys, and no user
 * authentication, because the push service runs while the device is locked.
 *
 * A sealed value is the 12-byte IV followed by the ciphertext and its tag.
 *
 * @param alias the Keystore alias of the key; created on first use.
 */
class KeystoreCipher(private val alias: String = ALIAS) : ValueCipher {
    override fun encrypt(plain: ByteArray, context: ByteArray): ByteArray {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD(context)
        return cipher.iv + cipher.doFinal(plain)
    }

    override fun decrypt(sealed: ByteArray, context: ByteArray): ByteArray {
        require(sealed.size > IV_BYTES) { "sealed value too short" }
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(TAG_BITS, sealed, 0, IV_BYTES))
        cipher.updateAAD(context)
        return cipher.doFinal(sealed, IV_BYTES, sealed.size - IV_BYTES)
    }

    /** Deletes the key; every value sealed with it becomes unreadable. */
    fun deleteKey() {
        val store = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        if (store.containsAlias(alias)) store.deleteEntry(alias)
    }

    @Synchronized
    private fun key(): SecretKey {
        val store = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        (store.getKey(alias, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(KEY_BITS)
                .setRandomizedEncryptionRequired(true)
                .build(),
        )
        return generator.generateKey()
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val ALIAS = "push_store"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_BYTES = 12
        const val TAG_BITS = 128
        const val KEY_BITS = 256
    }
}

/**
 * [KeyValueStore] over private [SharedPreferences]; `commit`, so the service and the worker see
 * each other's writes at once. The backup rules of M5-03 exclude every domain (D-M5-03-1).
 */
class SharedPreferencesValues(private val preferences: SharedPreferences) : KeyValueStore {
    override fun get(key: String): String? = preferences.getString(key, null)

    override fun put(key: String, value: String?) {
        val editor = preferences.edit()
        if (value == null) editor.remove(key) else editor.putString(key, value)
        editor.commit()
    }

    override fun keys(): Set<String> = preferences.all.keys.toSet()

    /** The store of the app. */
    companion object {
        /** The preferences file of the push store. */
        const val FILE = "push_store"

        /** The push store's preferences of [context]. */
        fun of(context: Context): SharedPreferencesValues =
            SharedPreferencesValues(context.getSharedPreferences(FILE, Context.MODE_PRIVATE))
    }
}

/**
 * The registration of one instance, as `registerPush` returned it and the app handed it over.
 *
 * [toString] names no value, so a log line never carries the preview token.
 *
 * @property serverUrl the server address of the instance.
 * @property registrationId the ID of the push registration.
 * @property previewToken the preview token (13 6.4), shown once by the server.
 */
class PushRegistration(val serverUrl: String, val registrationId: String, val previewToken: String) {
    override fun toString(): String = "PushRegistration(<redacted>)"
}

/** The display counters since the app last took them: duration buckets and placeholders. */
data class DisplayCounts(val buckets: List<Int>, val placeholders: Int)

/**
 * What the push service and its worker keep between a push and the app (docs/spec/
 * 15-client-architecture.md 10.2; brief M5-05 Design notes 2 and 4; D-M5-05-4).
 *
 * Every value is sealed with [cipher] and bound to its key; key names that would carry an ID
 * use its SHA-256, so neither a server address nor a thread ID is readable in the file. It keeps
 * per instance the server address, the registration ID and the preview token, and globally the
 * signed-in instance, `notify.preview_mode`, `client.push.fetch_timeout`, the last `DPoP-Nonce`
 * per server, the newest `nid` per thread and the display counters; no content and no names.
 *
 * @param values the plain storage.
 * @param cipher the encryption of every value.
 */
class PushStore(private val values: KeyValueStore, private val cipher: ValueCipher) {
    /** Keeps [registration] for [instanceId], replacing the earlier one. */
    fun saveRegistration(instanceId: String, registration: PushRegistration) =
        locked {
            val h = hash(instanceId)
            write("reg.$h.server_url", registration.serverUrl)
            write("reg.$h.registration_id", registration.registrationId)
            write("reg.$h.preview_token", registration.previewToken)
        }

    /** The registration of [instanceId], or null when none is complete. */
    fun registration(instanceId: String): PushRegistration? =
        locked {
            val h = hash(instanceId)
            val server = read("reg.$h.server_url")
            val id = read("reg.$h.registration_id")
            val token = read("reg.$h.preview_token")
            if (server == null || id == null || token == null) null else PushRegistration(server, id, token)
        }

    /**
     * Ends the account of [instanceId]: its registration, and the settings, nonces and newest
     * `nid`s when it is the signed-in instance.
     */
    fun deleteRegistration(instanceId: String) =
        locked {
            val h = hash(instanceId)
            listOf("server_url", "registration_id", "preview_token").forEach { values.put("reg.$h.$it", null) }
            if (read(SIGNED_IN) == instanceId) {
                listOf(SIGNED_IN, PREVIEW_MODE, FETCH_TIMEOUT).forEach { values.put(it, null) }
                values.keys().filter { it.startsWith("nonce.") || it.startsWith("newest.") }.forEach { values.put(it, null) }
            }
        }

    /** Writes what the app hands over at start and on every change of `notify.preview_mode`. */
    fun writeSettings(instanceId: String, previewMode: String, fetchTimeoutMillis: Long) =
        locked {
            write(SIGNED_IN, instanceId)
            write(PREVIEW_MODE, previewMode)
            write(FETCH_TIMEOUT, fetchTimeoutMillis.toString())
        }

    /** The signed-in instance, or null. */
    fun signedInInstance(): String? = locked { read(SIGNED_IN) }

    /** `notify.preview_mode` as last written, or null before the first start after a login. */
    fun previewMode(): String? = locked { read(PREVIEW_MODE) }

    /** `client.push.fetch_timeout` in milliseconds as last written, or null. */
    fun fetchTimeoutMillis(): Long? = locked { read(FETCH_TIMEOUT)?.toLongOrNull()?.takeIf { it > 0 } }

    /** The last `DPoP-Nonce` of the server [origin], or null. */
    fun nonce(origin: String): String? = locked { read("nonce.${hash(origin)}") }

    /** Keeps [nonce] as the newest of the server [origin]. */
    fun rememberNonce(origin: String, nonce: String) = locked { write("nonce.${hash(origin)}", nonce) }

    /** Records [nid] as the newest push of the notification [tag]. */
    fun setNewest(tag: String, nid: String) = locked { write("newest.${hash(tag)}", nid) }

    /** The newest push of the notification [tag], or null. */
    fun newest(tag: String): String? = locked { read("newest.${hash(tag)}") }

    /** Forgets the newest push of [tag] (its notification was cleared). */
    fun clearNewest(tag: String) = locked { values.put("newest.${hash(tag)}", null) }

    /**
     * Counts one push: [seconds] from receipt to the final display into the bucket of
     * [DisplayBuckets.BOUNDS] (a value above the top bound in the top one), and whether the
     * placeholder stayed.
     */
    fun record(seconds: Double, placeholder: Boolean) =
        locked {
            if (seconds.isFinite() && seconds >= 0) {
                val buckets = buckets().toMutableList()
                buckets[DisplayBuckets.indexOf(seconds)] += 1
                write(BUCKETS, buckets.joinToString(","))
            }
            if (placeholder) write(PLACEHOLDERS, ((read(PLACEHOLDERS)?.toIntOrNull() ?: 0) + 1).toString())
        }

    /** Returns the counts since the last call and resets them. */
    fun takeCounts(): DisplayCounts =
        locked {
            val counts = DisplayCounts(buckets(), read(PLACEHOLDERS)?.toIntOrNull() ?: 0)
            values.put(BUCKETS, null)
            values.put(PLACEHOLDERS, null)
            counts
        }

    private fun buckets(): List<Int> {
        val stored = read(BUCKETS)?.split(',')?.map { it.toIntOrNull() ?: 0 }.orEmpty()
        return List(DisplayBuckets.BOUNDS.size) { stored.getOrElse(it) { 0 } }
    }

    private fun write(key: String, value: String) {
        val sealed = cipher.encrypt(value.toByteArray(Charsets.UTF_8), key.toByteArray(Charsets.UTF_8))
        values.put(key, Base64.getEncoder().encodeToString(sealed))
    }

    // A value that does not open (changed, or its key was lost with a restore) counts as absent.
    private fun read(key: String): String? {
        val stored = values.get(key) ?: return null
        return runCatching {
            String(cipher.decrypt(Base64.getDecoder().decode(stored), key.toByteArray(Charsets.UTF_8)), Charsets.UTF_8)
        }.getOrNull()
    }

    // The service, the worker and the Pigeon host run in one process; one lock keeps each
    // read-modify-write whole.
    private inline fun <T> locked(body: () -> T): T = synchronized(LOCK) { body() }

    /** The key names and the shared lock. */
    companion object {
        private val LOCK = Any()
        private const val SIGNED_IN = "signed_in_instance"
        private const val PREVIEW_MODE = "preview_mode"
        private const val FETCH_TIMEOUT = "fetch_timeout_ms"
        private const val BUCKETS = "counters.buckets"
        private const val PLACEHOLDERS = "counters.placeholders"

        /** The SHA-256 of [value] in lower-case hex, the key form of an ID or address. */
        fun hash(value: String): String =
            MessageDigest.getInstance("SHA-256").digest(value.toByteArray(Charsets.UTF_8)).joinToString("") { "%02x".format(it) }
    }
}

/** The bucket bounds of `client.push_display.duration` (docs/observability/signals.md 4.7). */
object DisplayBuckets {
    /** The upper bounds in seconds, the only ones the server accepts for an app histogram. */
    val BOUNDS = listOf(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0)

    /** The bucket of [seconds]; above the top bound the top bucket, as the app's collector. */
    fun indexOf(seconds: Double): Int = BOUNDS.indexOfFirst { seconds <= it }.takeIf { it >= 0 } ?: (BOUNDS.size - 1)
}
