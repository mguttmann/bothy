// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.Context
import java.security.SecureRandom
import java.util.Base64
import java.util.Random

/**
 * The fake FCM token of a debug build with `e2ePush=true` (brief M5-11 Design note 2):
 * random, made at the first request and kept in the app's private storage, so every start
 * registers the same token and a new install a new one. The backup rules of M5-03 exclude every
 * file of the app (D-M5-03-1), so no restore copies it to another install.
 *
 * The token is [PREFIX] and the unpadded base64url of 32 random bytes: printable ASCII without
 * white space, which `registerPush` requires of an `fcm` token, and short enough for the
 * smallest allowed `push.registration_field_max_bytes`.
 *
 * @param values the private storage of the token.
 * @param random the source of the token's bytes; a seeded one in the JVM tests.
 */
class E2ePushTokens(private val values: KeyValueStore, private val random: Random = SecureRandom()) {
    /** The token of this install; the first call makes and stores it. */
    fun token(): String =
        synchronized(LOCK) {
            values.get(KEY) ?: create().also { values.put(KEY, it) }
        }

    private fun create(): String {
        val bytes = ByteArray(TOKEN_BYTES).also(random::nextBytes)
        return PREFIX + Base64.getUrlEncoder().withoutPadding().encodeToString(bytes)
    }

    /** The storage names and the store of the app. */
    companion object {
        /** The private preferences file of the token. */
        const val FILE = "e2e_push"

        /** The key of the token in [FILE]. */
        const val KEY = "fcm_token"

        /** The start of every fake token, so the server's records show where a token came from. */
        const val PREFIX = "e2e-"

        private const val TOKEN_BYTES = 32

        // Two first requests at once must not make two tokens.
        private val LOCK = Any()

        /** The token store of the app [context]. */
        fun of(context: Context): E2ePushTokens =
            E2ePushTokens(SharedPreferencesValues(context.getSharedPreferences(FILE, Context.MODE_PRIVATE)))
    }
}
