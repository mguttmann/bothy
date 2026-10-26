// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

/**
 * The push backend of a build without a Firebase configuration (15 10.2; brief M5-05 Design
 * note 4; D-M5-05-5): no push, and no Firebase or Play services code in the app. The build with
 * one compiles `src/firebase` instead, with the same declarations.
 */
object PushBackend {
    /** Whether this build receives pushes. */
    const val AVAILABLE = false

    /**
     * There is no token without FCM.
     *
     * @throws PushAndroidError `unavailable`, always.
     */
    @Suppress("RedundantSuspendModifier")
    suspend fun token(): String = throw PushAndroidError("unavailable", null, null)
}
