// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import com.google.firebase.messaging.FirebaseMessaging
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * The push backend of a build with a Firebase configuration (brief M5-05 Design note 4;
 * D-M5-05-5): FCM through the Firebase messaging library. The build without one compiles
 * `src/nofirebase` instead, with the same declarations.
 */
object PushBackend {
    /** Whether this build receives pushes. */
    const val AVAILABLE = true

    /**
     * The FCM registration token. Automatic initialisation is off in the manifest, so this
     * call, made only after the permission, is the first that reaches FCM.
     *
     * @throws PushAndroidError `unavailable` when FCM gives no token (no Play services, no
     *   network).
     */
    // `getToken` is deprecated in favour of `register` and `onRegistered` since firebase-messaging
    // 26, which deliver the token only through the service; the returned token is what the
    // registration needs at this moment, and the service publishes both callbacks.
    @Suppress("DEPRECATION")
    suspend fun token(): String =
        suspendCancellableCoroutine { continuation ->
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                val token = task.result.takeIf { task.isSuccessful && !it.isNullOrEmpty() }
                if (token != null) {
                    continuation.resume(token)
                } else {
                    continuation.resumeWithException(PushAndroidError("unavailable", null, null))
                }
            }
        }
}
