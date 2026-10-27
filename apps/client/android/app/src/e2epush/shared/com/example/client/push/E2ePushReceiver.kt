// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

/**
 * The debug-only entry of a push in the device E2E (brief M5-11 Design note 2): the runner's
 * relay reads what the server sent to the fake FCM and broadcasts its `data` here,
 *
 * ```
 * adb shell am broadcast -a <application ID>.E2E_PUSH \
 *   -n <application ID>/com.example.client.push.E2ePushReceiver --es data_b64 <base64 JSON>
 * ```
 *
 * and the receiver hands the map to [PushReceiver] exactly as the FCM service does with
 * `RemoteMessage.data`. Only a debug build with `bothyE2ePush=true` declares it; the manifest
 * guards it with `android.permission.DUMP`, which only the shell user (adb) and the system hold,
 * so no installed app can send to it.
 */
class E2ePushReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != action(context.packageName)) return
        val app = context.applicationContext
        val delivered = deliver(intent.getStringExtra(EXTRA_DATA)) { data -> PushReceiver(PushRuntime.of(app)).onMessage(data) }
        // The line names the reason, never the content.
        if (!delivered) Log.w(TAG, "dropped an E2E push: $EXTRA_DATA is missing or not a JSON object of strings")
    }

    /** The broadcast's names and the hand-over, which the JVM tests drive. */
    companion object {
        /** The string extra with the standard base64 of the JSON object of the message's `data`. */
        const val EXTRA_DATA = "data_b64"

        private const val TAG = "E2ePushReceiver"

        /**
         * The action of the broadcast.
         *
         * @param applicationId the application ID of the build (its package name).
         * @return `<applicationId>.E2E_PUSH`.
         */
        fun action(applicationId: String): String = "$applicationId.E2E_PUSH"

        /**
         * Decodes [dataB64] with [E2ePushData.decode] and hands the map to [handle].
         *
         * @param dataB64 the extra [EXTRA_DATA], or null when the broadcast has none.
         * @param handle receives the decoded `data`; the receiver passes [PushReceiver.onMessage].
         * @return whether [handle] was called; false for missing or malformed input, which is
         *   dropped without an exception.
         */
        fun deliver(dataB64: String?, handle: (Map<String, String>) -> Unit): Boolean {
            val data = E2ePushData.decode(dataB64) ?: return false
            handle(data)
            return true
        }
    }
}
