// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.Manifest
import android.content.Intent
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import com.example.client.push.PermissionAsker
import com.example.client.push.PushAndroidHost
import com.example.client.push.PushAndroidHostApi
import com.example.client.push.PushTaps
import io.flutter.embedding.android.FlutterFragmentActivity
import io.flutter.embedding.engine.FlutterEngine
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/**
 * The one activity of the app (brief M5-03 Design notes 2 to 5).
 *
 * A `FlutterFragmentActivity`, because the step-up prompt of the device keys (M5-01) needs a
 * `FragmentActivity`. It connects the Pigeon channel [AndroidPlatformApi] and the deep link
 * channel `bothy/deep_links`, receives the links that [LinkActivity] forwards (the OIDC
 * redirect goes to the waiting login, every other link to the router), ends a login whose
 * Custom Tab was closed, and hides the content in the recent apps screen ([RecentsProtection]).
 * Since M5-05 it also connects the push channel [PushAndroidHostApi], asks for the notification
 * permission for it and hands the taps of push notifications to the app ([PushTaps]).
 * It also takes the shares that [ShareActivity] forwards to the process's [ShareReceiver] and
 * connects that receiver to the Pigeon channels [ShareAndroidHostApi] and
 * [ShareAndroidFlutterApi] (brief M5-06 Design note 6), and the clipboard of block copies
 * [RichClipboardChannel] (brief M4-48). A debug build also connects the text input probe of the
 * phone integration tests ([TextInputProbe], DICTATION-MOBILE).
 */
class MainActivity : FlutterFragmentActivity(), PermissionAsker {
    private val login = CustomTabsLogin { this }
    private val recents = RecentsProtection()
    private var deepLinks: DeepLinkChannel? = null
    private var push: PushAndroidHost? = null
    private val share by lazy { ShareIntents.receiver(applicationContext) }
    private var shareNotify: (() -> Unit)? = null
    private var firstStart = true
    private var permissionAnswer: CompletableDeferred<Boolean>? = null

    // Registered before the activity starts, as the Activity Result API requires.
    private val notificationPermission =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            permissionAnswer?.complete(granted)
            permissionAnswer = null
        }

    /** The recent apps protection of this activity, read by the instrumented tests. */
    val recentsProtection: RecentsProtection
        get() = recents

    override fun onCreate(savedInstanceState: Bundle?) {
        // A recreated activity must not open the link of its first start again.
        firstStart = savedInstanceState == null
        super.onCreate(savedInstanceState)
        recents.onCreate(this)
        // A cold start by a share; like a link, a recreated or relaunched task has taken it.
        if (firstStart && !fromHistory(intent)) ShareIntents.requestOf(intent)?.let(share::receive)
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
                secureWindow = { recents.setTerminalSecure(this, it) },
                keyboard = { hardwareKeyboard(resources.configuration) },
            ),
        )
        val host = PushAndroidHost(applicationContext, messenger, this)
        push = host
        PushAndroidHostApi.setUp(messenger, host)
        ShareAndroidHostApi.setUp(messenger, share)
        val shareEvents = ShareAndroidFlutterApi(messenger)
        val notify = {
            CoroutineScope(Dispatchers.Main).launch {
                try {
                    shareEvents.onShareReceived()
                } catch (_: ShareAndroidPigeonError) {
                    // No listening app: the share waits for its next takePendingShare.
                }
            }
            Unit
        }
        shareNotify = notify
        share.attach(notify)
        RichClipboardChannel(messenger, applicationContext)
        // Debug builds only: the probe of the phone integration tests (DICTATION-MOBILE).
        TextInputProbe.attach(messenger, this)
        val links = DeepLinkChannel(messenger)
        deepLinks = links
        // A cold start by a link: LinkActivity started this activity with the link as data.
        // A task relaunched from the recent apps keeps its first intent; its link was opened
        // then and must not open again.
        if (firstStart && !fromHistory(intent)) {
            linkOf(intent)?.let(links::open)
            PushTaps.of(intent)?.let(PushTaps::deliver)
        }
    }

    override fun cleanUpFlutterEngine(flutterEngine: FlutterEngine) {
        PushAndroidHostApi.setUp(flutterEngine.dartExecutor.binaryMessenger, null)
        push?.detach()
        push = null
        // The share receiver outlives this engine; it must neither call nor keep its messenger.
        shareNotify?.let(share::detach)
        shareNotify = null
        ShareAndroidHostApi.setUp(flutterEngine.dartExecutor.binaryMessenger, null)
        super.cleanUpFlutterEngine(flutterEngine)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        PushTaps.of(intent)?.let {
            PushTaps.deliver(it)
            return
        }
        ShareIntents.requestOf(intent)?.let {
            share.receive(it)
            return
        }
        val link = linkOf(intent) ?: return
        if (!login.deliver(link)) deepLinks?.open(link)
    }

    /** Shows the system prompt for `POST_NOTIFICATIONS` (API 33 and later, D-M5-05-6). */
    override suspend fun ask(): Boolean {
        permissionAnswer?.let { return it.await() }
        val answer = CompletableDeferred<Boolean>()
        permissionAnswer = answer
        notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        return answer.await()
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

    /** A physical keyboard is attached and not hidden (brief M5-12, 01 screen M8). */
    private fun hardwareKeyboard(config: Configuration): Boolean =
        config.keyboard != Configuration.KEYBOARD_NOKEYS &&
            config.hardKeyboardHidden == Configuration.HARDKEYBOARDHIDDEN_NO

    private fun linkOf(intent: Intent?): String? =
        intent?.takeIf { it.action == Intent.ACTION_VIEW }?.dataString

    private fun fromHistory(intent: Intent?): Boolean =
        intent != null && intent.flags and Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY != 0
}
