// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.os.Build
import android.os.SystemClock

/**
 * The host side of the Pigeon channel [AndroidPlatformApi] (pigeons/android_platform_api.dart):
 * the Custom Tabs login, the Credential Manager passkeys, the notification channels, the
 * device name and the time since boot of the app lock.
 */
class AndroidPlatform(
    private val login: CustomTabsLogin,
    private val passkeys: CredentialManagerPasskeys,
    private val channels: NotificationChannelRegistrar,
) : AndroidPlatformApi {
    override suspend fun authenticate(url: String, callbackScheme: String): String =
        login.authenticate(url, callbackScheme)

    override fun passkeysSupported(): Boolean = passkeys.supported()

    override suspend fun passkeyAssertion(requestOptionsJson: String): String =
        passkeys.assertion(requestOptionsJson)

    override suspend fun passkeyRegistration(creationOptionsJson: String): String =
        passkeys.registration(creationOptionsJson)

    override fun registerNotificationChannels(channels: List<AndroidNotificationChannel>) =
        this.channels.register(channels)

    /** "Manufacturer Model", or the model alone when it already starts with the manufacturer. */
    override fun deviceName(): String {
        val model = Build.MODEL.orEmpty()
        val maker = Build.MANUFACTURER.orEmpty().replaceFirstChar { it.uppercase() }
        return when {
            maker.isEmpty() -> model
            model.startsWith(maker, ignoreCase = true) -> model
            else -> "$maker $model"
        }.ifEmpty { "Android" }
    }

    /**
     * Milliseconds since boot including deep sleep (`CLOCK_BOOTTIME`), which the person cannot
     * set: the app lock counts the time a sleeping device spends with it (D-M5-08-8).
     */
    override fun sinceBootMillis(): Long = SystemClock.elapsedRealtime()
}
