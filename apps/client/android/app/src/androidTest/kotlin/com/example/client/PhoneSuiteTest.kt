// SPDX-License-Identifier: Apache-2.0

package com.example.client

import androidx.test.rule.ActivityTestRule
import dev.flutter.plugins.integration_test.FlutterTestRunner
import org.junit.Rule
import org.junit.runner.RunWith

/**
 * The phone integration tests (integration_test/phone_suite_test.dart) as an instrumented test
 * (FIX-android36-dds-start): tool/run_android_tests.sh builds the app with the suite as its entry
 * point, the runner of the integration_test plugin starts [MainActivity], waits for the results
 * of the Dart tests and reports each one as a JUnit test. No connection from the host to the
 * VM service of the app is needed, which on the API 36 emulator of the CI dropped with the adb
 * connection while the app started. It runs only in the build of the suite; every other run of
 * the instrumented tests leaves it out, because it would wait for results that never come.
 */
@RunWith(FlutterTestRunner::class)
class PhoneSuiteTest {
    @Rule @JvmField val rule = ActivityTestRule(MainActivity::class.java, true, false)
}
