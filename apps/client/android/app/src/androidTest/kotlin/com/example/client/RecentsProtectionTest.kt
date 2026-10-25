// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.os.Build
import android.view.WindowManager
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The recent apps screen shows no content (01 5.6 E6; brief M5-03 Design note 5): on API 33 and
 * later the app switches the recent apps picture off from the start; below, moving to the
 * background sets `FLAG_SECURE` on the window and coming back clears it.
 */
@RunWith(AndroidJUnit4::class)
class RecentsProtectionTest {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()

    @Test
    fun theBackgroundHidesTheContentFromTheRecentApps() {
        val activity = instrumentation.startActivitySync(launch(instrumentation.targetContext)) as MainActivity
        try {
            instrumentation.runOnMainSync {
                if (Build.VERSION.SDK_INT >= 33) {
                    assertEquals(RecentsProtection.Rule.NO_RECENTS_SCREENSHOT, activity.recentsProtection.rule)
                    assertTrue(activity.recentsProtection.recentsScreenshotDisabled)
                } else {
                    assertEquals(RecentsProtection.Rule.SECURE_IN_BACKGROUND, activity.recentsProtection.rule)
                    assertFalse("no FLAG_SECURE in front", secure(activity))
                }
            }

            // The activity leaves the foreground as when the person opens the recent apps.
            instrumentation.runOnMainSync { instrumentation.callActivityOnPause(activity) }
            instrumentation.runOnMainSync {
                if (Build.VERSION.SDK_INT >= 33) {
                    assertTrue(activity.recentsProtection.recentsScreenshotDisabled)
                } else {
                    assertTrue("FLAG_SECURE in the background", secure(activity))
                }
            }

            instrumentation.runOnMainSync { instrumentation.callActivityOnResume(activity) }
            instrumentation.runOnMainSync {
                if (Build.VERSION.SDK_INT < 33) assertFalse("FLAG_SECURE cleared", secure(activity))
            }
        } finally {
            activity.finish()
        }
    }

    @Test
    fun theRulePerApiLevel() {
        assertEquals(RecentsProtection.Rule.SECURE_IN_BACKGROUND, RecentsProtection(26).rule)
        assertEquals(RecentsProtection.Rule.SECURE_IN_BACKGROUND, RecentsProtection(32).rule)
        assertEquals(RecentsProtection.Rule.NO_RECENTS_SCREENSHOT, RecentsProtection(33).rule)
        assertEquals(RecentsProtection.Rule.NO_RECENTS_SCREENSHOT, RecentsProtection(36).rule)
    }

    private fun secure(activity: MainActivity): Boolean =
        activity.window.attributes.flags and WindowManager.LayoutParams.FLAG_SECURE != 0
}
