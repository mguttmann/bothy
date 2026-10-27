// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.content.Intent
import android.os.Build
import android.view.accessibility.AccessibilityNodeInfo
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.AndroidJUnit4
import com.google.android.apps.common.testing.accessibility.framework.AccessibilityCheckPreset
import com.google.android.apps.common.testing.accessibility.framework.AccessibilityCheckResult.AccessibilityCheckResultType
import com.google.android.apps.common.testing.accessibility.framework.AccessibilityHierarchyCheckResult
import com.google.android.apps.common.testing.accessibility.framework.Parameters
import com.google.android.apps.common.testing.accessibility.framework.uielement.AccessibilityHierarchyAndroid
import com.google.android.apps.common.testing.accessibility.framework.utils.contrast.BitmapImage
import java.util.Locale
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The checks of the Accessibility Test Framework on the phone screens (brief M5-10 Design note
 * 3; 01 8): each test starts the audit entry of the app (integration_test/a11y_audit/
 * audit_app.dart, built by tool/run_android_a11y_checks.sh) with one screen as the initial
 * route, waits until the screen's text is in the accessibility tree TalkBack reads, and runs
 * every check of the latest preset (touch target size, contrast with a screenshot, labels,
 * duplicate descriptions, clickable spans, traversal order and the rest) on that tree. Every
 * result of type ERROR fails the test, unless it is a known limitation of the Flutter engine
 * listed in [knownLimitations] with its reason (and in docs/quality/accessibility-mobile.md);
 * the test prints every result it lets pass.
 *
 * It runs only in the build of the audit entry; tool/run_android_tests.sh leaves it out.
 */
@RunWith(AndroidJUnit4::class)
class AccessibilityChecksTest {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()

    /** A result of a check that comes from the Flutter engine, not from the app. */
    private class Limitation(val check: String, val reason: String, val matches: (AccessibilityHierarchyCheckResult) -> Boolean)

    /**
     * Results of the checks that come from the Flutter engine, not from the app, each matched by
     * the check, the element and the API level, with the reason.
     */
    private val knownLimitations =
        listOf(
            Limitation(
                "SpeakableTextPresentCheck",
                "below API 28 the engine gives an empty text field no name: it sets the field's label " +
                    "as hint text only from API 28 on; the widget tests name every field",
            ) { result ->
                Build.VERSION.SDK_INT < 28 &&
                    result.element?.isEditable == true &&
                    result.element?.text?.toString().isNullOrEmpty()
            },
        )

    private fun audit(screen: String, text: String) {
        val errors = findings(screen, text)
        assertTrue(errors.joinToString("\n"), errors.isEmpty())
    }

    /** The results of type ERROR on [screen], without the known limitations. */
    private fun findings(screen: String, text: String): List<String> {
        val intent =
            Intent(instrumentation.targetContext, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                .putExtra("route", "/$screen")
        val activity: Activity = instrumentation.startActivitySync(intent)
        try {
            // The UiAutomation connection is an accessibility client: the engine builds its
            // semantics tree for it as for TalkBack.
            val automation = instrumentation.uiAutomation
            var root: AccessibilityNodeInfo? = null
            Patience.until("$screen: \"$text\" in the accessibility tree") {
                instrumentation.waitForIdleSync()
                root = automation.rootInActiveWindow
                root?.let { contains(it, text) } ?: false
            }
            val hierarchy =
                AccessibilityHierarchyAndroid.newBuilder(root!!, instrumentation.targetContext).build()
            val parameters = Parameters()
            automation.takeScreenshot()?.let { parameters.putScreenCapture(BitmapImage(it)) }
            val results =
                AccessibilityCheckPreset.getAccessibilityHierarchyChecksForPreset(AccessibilityCheckPreset.LATEST)
                    .flatMap { it.runCheckOnHierarchy(hierarchy, null, parameters) }
            val errors = mutableListOf<String>()
            for (result in results.filter { it.type == AccessibilityCheckResultType.ERROR }) {
                val check = result.sourceCheckClass.simpleName
                val line = "$screen: $check ${describe(result)}"
                val known = knownLimitations.firstOrNull { it.check == check && it.matches(result) }
                if (known != null) {
                    println("$line: known limitation: ${known.reason}")
                } else {
                    errors.add(line)
                }
            }
            assertTrue("$screen: the checks ran on an empty tree", hierarchy.activeWindow.allViews.size > 1)
            return errors
        } finally {
            activity.finish()
        }
    }

    private fun describe(result: AccessibilityHierarchyCheckResult): String {
        val element = result.element
        val name = element?.let { "\"${it.contentDescription ?: it.text ?: ""}\" ${it.boundsInScreen}" } ?: "no element"
        return "($name): ${result.getMessage(Locale.ENGLISH)}"
    }

    private fun contains(node: AccessibilityNodeInfo, text: String): Boolean {
        if (node.contentDescription?.contains(text) == true || node.text?.contains(text) == true) return true
        for (i in 0 until node.childCount) {
            val child = node.getChild(i) ?: continue
            if (contains(child, text)) return true
        }
        return false
    }

    /**
     * The checks find the gaps of a screen made to have them (a tappable square of 20 by 20
     * without a name), so a green screen is no empty run. The grey text on grey of that screen
     * is no finding: the text contrast check of the framework measures only text views, and
     * Flutter presents its text as plain views (docs/quality/accessibility-mobile.md, section
     * "Platform audits: known limitations"); the widget tests and the token gate check contrast.
     */
    @Test
    fun theChecksFindKnownGaps() {
        val errors = findings("known_gaps", "Audit self-check")
        println(errors.joinToString("\n"))
        assertTrue("no touch target finding", errors.any { it.contains("TouchTargetSizeCheck") })
        assertTrue("no label finding", errors.any { it.contains("SpeakableTextPresentCheck") })
    }

    @Test fun m1Spaces() = audit("m1_spaces", "Jump to")

    @Test fun m2PostList() = audit("m2_post_list", "New post")

    @Test fun m3HistoryRunning() = audit("m3_running", "Allow once")

    @Test fun m3HistoryAfterRun() = audit("m3_collapsed", "Please include expired threads.")

    @Test fun m4LargeComponents() = audit("m4_components", "Back to requests")

    @Test fun m5Recording() = audit("m5_recording", "Send voice message")

    @Test fun m6ShareSheet() = audit("m6_share_sheet", "Share to")

    @Test fun m7Offline() = audit("m7_offline", "Offline")

    @Test fun m11RunningNow() = audit("m11_running", "Running now")

    @Test fun m12RunGraph() = audit("m12_run_graph", "Run graph")

    @Test fun e1ServerAddress() = audit("e1_server", "Connect")

    @Test fun e2Login() = audit("e2_login", "Log in to")

    @Test fun e4PendingDevice() = audit("e4_pending", "Code")

    @Test fun e5Pairing() = audit("e5_pairing", "Code")

    @Test fun e6AppLock() = audit("e6_lock", "Unlock")

    @Test fun e6AppLockUnavailable() = audit("e6_lock_unavailable", "Check again")

    @Test fun e7SignedOut() = audit("e7_signed_out", "You are logged out")
}
