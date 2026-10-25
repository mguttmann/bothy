// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.Intent
import android.os.Bundle
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine

/**
 * The one activity of the app (brief M5-03 Design notes 2 to 5).
 *
 * A `FlutterFragmentActivity`, because the step-up prompt of the device keys (M5-01) needs a
 * `FragmentActivity`. It connects the Pigeon channel [AndroidPlatformApi] and the deep link
 * channel `bothy/deep_links`, receives the links that [LinkActivity] forwards (the OIDC
 * redirect goes to the waiting login, every other link to the router), ends a login whose
 * Custom Tab was closed, and hides the content in the recent apps screen ([RecentsProtection]).
 */
class MainActivity : FlutterFragmentActivity() {
    private val login = CustomTabsLogin { this }
    private val recents = RecentsProtection()
    private var deepLinks: DeepLinkChannel? = null
    private var firstStart = true

    /** The recent apps protection of this activity, read by the instrumented tests. */
    val recentsProtection: RecentsProtection
        get() = recents

    override fun onCreate(savedInstanceState: Bundle?) {
        // A recreated activity must not open the link of its first start again.
        firstStart = savedInstanceState == null
        super.onCreate(savedInstanceState)
        recents.onCreate(this)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        val messenger = flutterEngine.dartExecutor.binaryMessenger
        AndroidPlatformApi.setUp(
            messenger,
            AndroidPlatform(
                login = login,
                passkeys = CredentialManagerPasskeys { this },
                channels = NotificationChannelRegistrar(applicationContext),
            ),
        )
        val links = DeepLinkChannel(messenger)
        deepLinks = links
        // A cold start by a link: LinkActivity started this activity with the link as data.
        // A task relaunched from the recent apps keeps its first intent; its link was opened
        // then and must not open again.
        val fromHistory = intent.flags and Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY != 0
        if (firstStart && !fromHistory) linkOf(intent)?.let(links::open)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        val link = linkOf(intent) ?: return
        if (!login.deliver(link)) deepLinks?.open(link)
    }

    override fun onPause() {
        recents.onBackground(this)
        login.onLeftApp()
        super.onPause()
    }

    override fun onResume() {
        super.onResume()
        recents.onForeground(this)
        login.onBackInApp()
    }

    private fun linkOf(intent: Intent?): String? =
        intent?.takeIf { it.action == Intent.ACTION_VIEW }?.dataString
}
