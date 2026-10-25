// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.NotificationManager
import android.content.Intent
import android.os.SystemClock
import androidx.test.runner.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith

/**
 * After the app started once, the four channels of 01 10.3 are in the system settings with their
 * stable IDs, the names in the device language and the importance of brief M5-03 Design note 4
 * (PLAN row M5-03: visible in the system settings before the first push).
 */
@RunWith(AndroidJUnit4::class)
class NotificationChannelsTest {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context = instrumentation.targetContext
    private val manager = context.getSystemService(NotificationManager::class.java)

    @Test
    fun theFirstStartRegistersTheFourChannels() {
        val activity = instrumentation.startActivitySync(launch(context))
        try {
            // The Dart side registers them right after the engine started.
            Patience.until("the four channels are registered") {
                manager.notificationChannels.map { it.id }.containsAll(IDS)
            }
        } finally {
            activity.finish()
        }
        val channels = manager.notificationChannels.associateBy { it.id }
        // The emulators of the tests run in English; another device language gets its own names.
        val english = context.resources.configuration.locales[0].language == "en"
        for ((i, id) in IDS.withIndex()) {
            val channel = channels.getValue(id)
            if (english) assertEquals(id, NAMES_EN[i], channel.name.toString())
            if (channel.name.isNullOrEmpty()) fail("$id has no name")
            assertEquals(id, IMPORTANCE[i], channel.importance)
            if (channel.description.isNullOrEmpty()) fail("$id has no description")
        }
        val registrar = NotificationChannelRegistrar(context)
        assertEquals("mentions", registrar.channelOf("dm"))
        assertEquals("questions", registrar.channelOf("security"))
        assertEquals("runs", registrar.channelOf("run_failed"))
        assertEquals("messages", registrar.channelOf("later_kind"))
    }

    private companion object {
        val IDS = listOf("mentions", "questions", "messages", "runs")
        val NAMES_EN = listOf("Mentions", "Questions", "All messages", "Runs")
        val IMPORTANCE =
            listOf(
                NotificationManager.IMPORTANCE_HIGH,
                NotificationManager.IMPORTANCE_HIGH,
                NotificationManager.IMPORTANCE_DEFAULT,
                NotificationManager.IMPORTANCE_DEFAULT,
            )
    }
}

/** The intent that starts the app as the launcher does, in a task of its own. */
fun launch(context: android.content.Context): Intent =
    Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)

/**
 * The one upper bound of the waits of these tests: a condition is polled until it holds, and
 * only a run that is stuck reaches the bound (the rule of the repository's deterministic tests:
 * wait for a condition, never for a fixed time).
 */
object Patience {
    private const val BOUND_MS = 120_000L
    private const val POLL_MS = 50L

    /** Polls [condition] until it holds; fails with [what] when the bound passes first. */
    fun until(what: String, condition: () -> Boolean) {
        val end = SystemClock.uptimeMillis() + BOUND_MS
        while (!condition()) {
            if (SystemClock.uptimeMillis() > end) fail("timed out: $what")
            SystemClock.sleep(POLL_MS)
        }
    }
}
