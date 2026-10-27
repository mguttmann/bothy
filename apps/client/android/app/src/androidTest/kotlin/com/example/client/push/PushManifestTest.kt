// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.ComponentName
import android.content.Intent
import android.content.pm.PackageManager
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The merged manifest has the push service only in a build with a Firebase configuration
 * (brief M5-05 Design note 4; D-M5-05-5): without one it is declared disabled and nothing
 * receives `MESSAGING_EVENT`, also in the debug build of the device E2E (`e2ePush`, brief
 * M5-11), whose push backend is available without Firebase.
 */
@RunWith(AndroidJUnit4::class)
class PushManifestTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext

    // Only a build with a Firebase configuration links the Firebase messaging library.
    private val firebase = runCatching { Class.forName("com.google.firebase.messaging.FirebaseMessagingService") }.isSuccess

    @Test
    fun noFirebaseConfigNoService() {
        val component = ComponentName(context, PushMessagingService::class.java)
        val info = context.packageManager.getServiceInfo(component, PackageManager.MATCH_DISABLED_COMPONENTS)
        assertEquals(firebase, info.enabled)
        val receivers =
            context.packageManager.queryIntentServices(
                Intent("com.google.firebase.MESSAGING_EVENT").setPackage(context.packageName),
                0,
            )
        val ours = receivers.filter { it.serviceInfo.name == PushMessagingService::class.java.name }
        assertEquals(if (firebase) 1 else 0, ours.size)
    }
}
