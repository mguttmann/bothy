// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.Context
import com.example.client.NotificationChannelRegistrar
import devicekey.core.DeviceKeyCore
import devicekey.core.forContext

/**
 * What the push service, its worker and the Pigeon host share in the process (brief M5-05):
 * the store, the scheduler of the preview fetch, the device key, the connection and the clock.
 *
 * [of] gives the production runtime; the instrumented tests [install] one with a recording
 * scheduler, a connection that trusts their test CA and a clock they turn, so the same
 * receiver and worker code runs in both.
 *
 * @property context the application context.
 * @property store the encrypted push store.
 * @property scheduler starts the preview fetch of a push.
 * @property signer opens the `dpop` key of an instance, or null when there is none.
 * @property connector opens the HTTPS connection of a fetch.
 * @property clock the current time in Unix milliseconds.
 */
class PushRuntime(
    val context: Context,
    val store: PushStore,
    val scheduler: PreviewScheduler,
    val signer: (String) -> ProofSigner?,
    val connector: HttpsConnector,
    val clock: () -> Long,
) {
    /** A poster with the texts of the device language of this moment. */
    fun poster(): NotificationPoster {
        val channels = NotificationChannelRegistrar(context)
        return NotificationPoster(context, PushTexts.of(context), channels::channelOf)
    }

    /** The process-wide runtime. */
    companion object {
        @Volatile private var current: PushRuntime? = null

        /** The runtime of the process; the production one unless a test installed another. */
        fun of(context: Context): PushRuntime =
            current ?: synchronized(this) {
                current ?: production(context.applicationContext).also { current = it }
            }

        /** Replaces the runtime of the process; null returns to the production one. */
        fun install(runtime: PushRuntime?) {
            synchronized(this) { current = runtime }
        }

        /** The runtime of the app: the Keystore store, WorkManager, the device keys and the platform's TLS. */
        fun production(context: Context): PushRuntime {
            val keys = DeviceKeyCore.forContext(context) { null }
            return PushRuntime(
                context = context,
                store = PushStore(SharedPreferencesValues.of(context), KeystoreCipher()),
                scheduler = WorkManagerScheduler(context),
                signer = { scope -> DeviceKeyProofSigner.open(keys, scope) },
                connector = PlatformConnector,
                clock = System::currentTimeMillis,
            )
        }
    }
}
