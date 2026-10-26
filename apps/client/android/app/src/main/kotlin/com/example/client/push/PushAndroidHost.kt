// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.Manifest
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import io.flutter.plugin.common.BinaryMessenger
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

/** Asks the person for the notification permission; the main activity implements it. */
fun interface PermissionAsker {
    /** Shows the system prompt and returns whether the permission was granted. */
    suspend fun ask(): Boolean
}

/**
 * The taps of push notifications on their way to the app: kept until the app listens (a tap
 * that started the app), then handed over at once.
 */
object PushTaps {
    private val pending = mutableListOf<AndroidPushTap>()
    private var listener: ((AndroidPushTap) -> Unit)? = null

    /** The tap of [intent], or null when it is not one of a push notification. */
    fun of(intent: Intent?): AndroidPushTap? {
        if (intent?.action != NotificationPoster.ACTION_TAP) return null
        val instance = intent.getStringExtra(NotificationPoster.EXTRA_INSTANCE) ?: return null
        val thread = intent.getStringExtra(NotificationPoster.EXTRA_THREAD) ?: return null
        return AndroidPushTap(instance, thread, intent.getStringExtra(NotificationPoster.EXTRA_MESSAGE))
    }

    /** Hands [tap] to the listener, or keeps it until the app takes the pending taps. */
    @Synchronized
    fun deliver(tap: AndroidPushTap) {
        val current = listener
        if (current == null) pending += tap else current(tap)
    }

    /** Returns the kept taps and hands every later tap to [next]. */
    @Synchronized
    fun take(next: (AndroidPushTap) -> Unit): List<AndroidPushTap> {
        listener = next
        return pending.toList().also { pending.clear() }
    }

    /** The app stopped listening (its engine went away); later taps are kept again. */
    @Synchronized
    fun stop() {
        listener = null
    }
}

/**
 * The host side of the Pigeon channel `PushAndroidHostApi` (brief M5-05 Design note 4).
 *
 * The preview token and the registration ID go straight into the encrypted [PushStore]; Dart
 * never stores them. New FCM tokens ([PushTokens]) and taps ([PushTaps]) go to the app through
 * `PushAndroidFlutterApi`.
 *
 * @param context the application context.
 * @param messenger the engine's messenger.
 * @param permission asks for the notification permission (API 33 and later).
 */
class PushAndroidHost(
    private val context: Context,
    messenger: BinaryMessenger,
    private val permission: PermissionAsker,
) : PushAndroidHostApi {
    private val flutter = PushAndroidFlutterApi(messenger)
    private val scope = CoroutineScope(Dispatchers.Main)
    private val runtime get() = PushRuntime.of(context)

    init {
        PushTokens.listen { token -> scope.launch { runCatching { flutter.onNewToken(token) } } }
    }

    /** Stops forwarding tokens and taps; the engine is going away. */
    fun detach() {
        PushTokens.listen(null)
        PushTaps.stop()
    }

    override fun isAvailable(): Boolean = PushBackend.AVAILABLE

    override fun appId(): String = context.packageName

    override fun isAuthorized(): Boolean {
        val enabled = context.getSystemService(NotificationManager::class.java).areNotificationsEnabled()
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return enabled
        return enabled && context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
    }

    override suspend fun requestAuthorization(): Boolean {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU && !isAuthorized()) permission.ask()
        return isAuthorized()
    }

    override suspend fun token(): String = PushBackend.token()

    override fun saveRegistration(registration: AndroidPushRegistration) {
        runtime.store.saveRegistration(
            registration.instanceId,
            PushRegistration(registration.serverUrl, registration.registrationId, registration.previewToken),
        )
    }

    override fun registrationId(instanceId: String): String? = runtime.store.registration(instanceId)?.registrationId

    override fun deleteRegistration(instanceId: String) {
        runtime.store.deleteRegistration(instanceId)
        // The account ended: its notifications must not stay in the drawer.
        runtime.poster().clearAll()
    }

    override fun writeSettings(instanceId: String, previewMode: String, fetchTimeoutMillis: Long) {
        runtime.store.writeSettings(instanceId, previewMode, fetchTimeoutMillis)
    }

    override fun takeDisplayCounters(): AndroidPushCounters {
        val counts = runtime.store.takeCounts()
        return AndroidPushCounters(DisplayBuckets.BOUNDS, counts.buckets.map(Int::toLong), counts.placeholders.toLong())
    }

    override fun takePendingTaps(): List<AndroidPushTap> =
        PushTaps.take { tap -> scope.launch { runCatching { flutter.onTap(tap) } } }
}
