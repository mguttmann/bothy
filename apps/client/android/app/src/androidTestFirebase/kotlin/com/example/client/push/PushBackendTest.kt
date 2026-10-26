// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import androidx.test.runner.AndroidJUnit4
import com.google.android.gms.tasks.Tasks
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The token of a finished FCM token task (a build with a Firebase configuration): a failed
 * task (no Play services, no network) gives no token instead of throwing inside the listener,
 * so [PushBackend.token] reports `unavailable` and the app keeps running.
 */
@RunWith(AndroidJUnit4::class)
class PushBackendTest {
    @Test
    fun aFailedTaskGivesNoToken() {
        assertNull(PushBackend.tokenOf(Tasks.forException(java.io.IOException("SERVICE_NOT_AVAILABLE"))))
    }

    @Test
    fun anEmptyTokenGivesNoToken() {
        assertNull(PushBackend.tokenOf(Tasks.forResult("")))
    }

    @Test
    fun aTokenIsReturned() {
        assertEquals("fcm-token", PushBackend.tokenOf(Tasks.forResult("fcm-token")))
    }
}
