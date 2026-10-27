// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.os.Build
import android.view.WindowManager

/**
 * Keeps the content out of the recent apps screen (01 5.6 E6; 15 7.2; brief M5-03 Design note 5;
 * docs/decisions.d/M5-03.md D-M5-03-5), with the narrowest rule of each API level.
 *
 * API 33 and later: `setRecentsScreenshotEnabled(false)` from the start, so the system takes no
 * picture of the app for the recent apps screen; screenshots of the open app stay possible.
 * API 26 to 32 have no such switch: the window gets `FLAG_SECURE` when the app leaves the
 * foreground (`onPause`, before the system takes the picture) and loses it in `onResume`, so
 * the flag blocks screenshots and screen recording only while the app is not in front. The
 * privacy cover of the Flutter side is drawn on `inactive` as well.
 *
 * While a terminal is open (brief M5-12 Design note 4, docs/decisions.d/M5-12.md D-M5-12-3) the
 * window keeps `FLAG_SECURE` on every API level, in front and behind: no screenshot, no screen
 * recording and no recent apps picture of terminal content. Closing the terminal clears the flag
 * only where the rule above does not need it.
 *
 * @param sdk the API level of the device.
 */
class RecentsProtection(private val sdk: Int = Build.VERSION.SDK_INT) {
    /** The rule of one API level. */
    enum class Rule {
        /** API 33 and later: no recent apps picture at all. */
        NO_RECENTS_SCREENSHOT,

        /** API 26 to 32: `FLAG_SECURE` while the app is not in front. */
        SECURE_IN_BACKGROUND,
    }

    /** The rule of this device. */
    val rule: Rule = if (sdk >= TIRAMISU) Rule.NO_RECENTS_SCREENSHOT else Rule.SECURE_IN_BACKGROUND

    /** Whether the recent apps picture is switched off (the rule of API 33 and later). */
    var recentsScreenshotDisabled: Boolean = false
        private set

    /** Whether a terminal asked for `FLAG_SECURE` ([setTerminalSecure]). */
    var terminalSecure: Boolean = false
        private set

    private var inBackground = false

    /** Applies the rule of API 33 and later once at the start of [activity]. */
    fun onCreate(activity: Activity) {
        if (rule == Rule.NO_RECENTS_SCREENSHOT && Build.VERSION.SDK_INT >= TIRAMISU) {
            activity.setRecentsScreenshotEnabled(false)
            recentsScreenshotDisabled = true
        }
    }

    /** [activity] leaves the foreground. */
    fun onBackground(activity: Activity) {
        inBackground = true
        if (rule == Rule.SECURE_IN_BACKGROUND) {
            activity.window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
    }

    /** [activity] is in front again. */
    fun onForeground(activity: Activity) {
        inBackground = false
        if (rule == Rule.SECURE_IN_BACKGROUND && !terminalSecure) {
            activity.window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
    }

    /** A terminal of [activity] opens ([secure]) or closes. */
    fun setTerminalSecure(activity: Activity, secure: Boolean) {
        terminalSecure = secure
        if (secure) {
            activity.window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        } else if (!(rule == Rule.SECURE_IN_BACKGROUND && inBackground)) {
            activity.window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
    }

    private companion object {
        const val TIRAMISU = 33
    }
}
