// SPDX-License-Identifier: Apache-2.0

package com.example.client

import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.Parameterized
import pl.leancode.patrol.PatrolJUnitRunner

/**
 * The bridge of the `patrol` device tests (brief M5-06 Tests): `patrol test` builds the app with
 * a bundle of the Dart tests as its entry point, and this class lists them and runs each one as
 * a JUnit test of its own. It runs only under `patrol test`; `tool/run_android_tests.sh`
 * leaves it out of the plain instrumented tests, whose app has no test bundle.
 */
@RunWith(Parameterized::class)
class MainActivityTest(private val dartTestName: String) {
    /** Runs the Dart test [dartTestName] in the app and fails with its failure. */
    @Test
    fun runDartTest() {
        val instrumentation = InstrumentationRegistry.getInstrumentation() as PatrolJUnitRunner
        instrumentation.runDartTest(dartTestName)
    }

    companion object {
        /** Starts the app and returns the names of the bundled Dart tests. */
        @JvmStatic
        @Parameterized.Parameters(name = "{0}")
        fun testCases(): Array<Any> {
            val instrumentation = InstrumentationRegistry.getInstrumentation() as PatrolJUnitRunner
            instrumentation.setUp(MainActivity::class.java)
            instrumentation.waitForPatrolAppService()
            return instrumentation.listDartTests()
        }
    }
}
