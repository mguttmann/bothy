// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * The push backend of a debug build with the Gradle property `e2ePush=true` (brief M5-11
 * Design note 2): push without Firebase for the device E2E against the Compose stack. The app
 * asks for the permission and registers this fake FCM token with provider `fcm` as in the build
 * with Firebase; the server sends to the stack's fake FCM, and the runner hands the captured
 * `data` to [E2ePushReceiver], which calls the same [PushReceiver] as the FCM service. Release
 * and profile builds compile `src/firebase` or `src/nofirebase` instead, with the same
 * declarations.
 */
object PushBackend {
    /** Whether this build receives pushes. */
    const val AVAILABLE = true

    /**
     * The fake FCM token of this install, made at the first call (see [E2ePushTokens]).
     *
     * @throws PushAndroidError `unavailable` when the app process has no application context
     *   yet ([E2ePushContext] did not start).
     */
    suspend fun token(): String {
        val context = E2ePushContext.application() ?: throw PushAndroidError("unavailable", null, null)
        // The first call writes the token file; keep the disk off the main thread.
        return withContext(Dispatchers.IO) { E2ePushTokens.of(context).token() }
    }
}
