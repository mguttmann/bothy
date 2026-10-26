// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import androidx.test.runner.AndroidJUnit4
import com.google.firebase.messaging.RemoteMessage
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The FCM service itself (a build with a Firebase configuration): a constructed
 * [RemoteMessage] passed as `onMessageReceived` gets it posts the placeholder at once and
 * schedules the fetch, without a network call.
 */
@RunWith(AndroidJUnit4::class)
class PushMessagingServiceTest {
    private val fixture = PushFixture()

    @After
    fun tearDown() = fixture.close()

    @Test
    fun aRemoteMessagePostsThePlaceholder() {
        fixture.signIn()
        val message = RemoteMessage.Builder("sender@example.com").setData(fixture.data("dm")).build()
        assertEquals(PushReceiver.Handled.SCHEDULED, PushMessagingService.handle(fixture.context, message))
        assertNotNull(fixture.shown("0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11"))
        assertTrue(fixture.received().isEmpty())
        assertEquals(1, fixture.scheduled.size)
    }
}
