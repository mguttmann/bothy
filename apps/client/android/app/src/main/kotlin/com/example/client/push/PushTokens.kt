// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.os.Handler
import android.os.Looper

/**
 * The FCM token stream to Dart (brief M5-05 Design note 4): the service's `onNewToken` publishes
 * here, and the Pigeon host forwards it to the app while an engine listens. Without a listener
 * the token is dropped: the app asks for the token at its next start and registers a changed
 * one then.
 */
object PushTokens {
    @Volatile private var listener: ((String) -> Unit)? = null
    private val main = Handler(Looper.getMainLooper())

    /** Sets the listener of new tokens, or removes it with null. */
    fun listen(listener: ((String) -> Unit)?) {
        this.listener = listener
    }

    /** Hands [token] to the listener on the main thread. */
    fun publish(token: String) {
        main.post { listener?.invoke(token) }
    }
}
