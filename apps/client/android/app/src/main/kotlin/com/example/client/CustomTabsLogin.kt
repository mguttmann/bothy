// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Intent
import android.net.Uri
import androidx.browser.customtabs.CustomTabsClient
import androidx.browser.customtabs.CustomTabsIntent
import kotlinx.coroutines.CancellableContinuation
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * The OIDC login in a Custom Tab (15 8; RFC 8252; brief M5-03 Design note 2): never a web view.
 *
 * [authenticate] opens the URL in a Custom Tab, or in the default browser when no Custom Tabs
 * provider is installed, and waits until [deliver] gets the redirect with the expected scheme
 * ([LinkActivity] forwards it). When the person comes back to the app without the redirect
 * (closes the tab, switches back), [onBackInApp] ends the login with `cancelled`.
 *
 * @param activity returns the activity that opens the tab, or null while there is none.
 */
class CustomTabsLogin(private val activity: () -> Activity?) {
    private var waiting: CancellableContinuation<String>? = null
    private var scheme: String? = null
    private var leftApp = false

    /**
     * Opens [url] and returns the redirect URL whose scheme is [callbackScheme]; throws a
     * [FlutterError] `cancelled` when the person returns without it, `unavailable` when no
     * browser can open the URL. A second login ends the first with `cancelled`.
     */
    suspend fun authenticate(url: String, callbackScheme: String): String {
        val host = activity() ?: throw FlutterError("unavailable")
        cancel()
        return suspendCancellableCoroutine { continuation ->
            waiting = continuation
            scheme = callbackScheme
            leftApp = false
            continuation.invokeOnCancellation { if (waiting === continuation) clear() }
            try {
                open(host, Uri.parse(url))
            } catch (_: ActivityNotFoundException) {
                clear()
                continuation.resumeWithException(FlutterError("unavailable"))
            }
        }
    }

    /** Takes [link] when it is the redirect a login waits for; returns whether it did. */
    fun deliver(link: String): Boolean {
        val continuation = waiting ?: return false
        if (!Uri.parse(link).scheme.equals(scheme, ignoreCase = true)) return false
        clear()
        continuation.resume(link)
        return true
    }

    /** The app went to the background, usually because the tab opened. */
    fun onLeftApp() {
        if (waiting != null) leftApp = true
    }

    /**
     * The app is in front again. Android delivers a redirect (`onNewIntent`) before
     * `onResume`, so a login that still waits here came back without one.
     */
    fun onBackInApp() {
        if (leftApp) cancel()
    }

    private fun cancel() {
        val continuation = waiting ?: return
        clear()
        continuation.resumeWithException(FlutterError("cancelled"))
    }

    private fun clear() {
        waiting = null
        scheme = null
        leftApp = false
    }

    private fun open(host: Activity, uri: Uri) {
        if (CustomTabsClient.getPackageName(host, null) != null) {
            CustomTabsIntent.Builder().build().launchUrl(host, uri)
        } else {
            host.startActivity(
                Intent(Intent.ACTION_VIEW, uri).addCategory(Intent.CATEGORY_BROWSABLE),
            )
        }
    }
}
